package autobrrmediahelper

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/gocolly/colly/v2"
	_ "github.com/mattn/go-sqlite3"
	"log/slog"
	url2 "net/url"
	"regexp"
	"strconv"
)

type MediaType string

const (
	Movie  MediaType = "movie"
	TVShow MediaType = "tv"
)

type Media struct {
	Id     string
	Title  string
	Year   int
	Rank   int
	Rating float64
	Type   MediaType
	Url    string
}

const (
	IMDBPopularMoviesUrl  = "https://www.imdb.com/chart/moviemeter/"
	IMDBPopularTVShowsUrl = "https://www.imdb.com/chart/tvmeter/"
	IMDBSearchUrl         = "https://www.imdb.com/search/title/?title=%s&release_date=%d-01-01,%d-12-31"
)

type MediaManager struct {
	db     *sql.DB
	config *Config
}

func NewMediaManager(config *Config) *MediaManager {
	db, err := sql.Open("sqlite3", config.DBName)
	if err != nil {
		panic(err)
	}
	return &MediaManager{
		db:     db,
		config: config,
	}
}

func (manager *MediaManager) Init() error {
	tx, err := manager.db.Begin()
	fail := func(err error) error {
		return fmt.Errorf("could not initialize db: %w", err)
	}
	defer tx.Rollback()
	// id, title, year, rank, media_type, url, rating, metadata_updated_at
	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS media (id TEXT PRIMARY KEY, title TEXT NOT NULL, YEAR INTEGER, rank INTEGER, media_type TEXT CHECK( media_type IN ('movie','tv')) NOT NULL, url TEXT NOT NULL, rating REAL NOT NULL, metadata_updated_at TIMESTAMP NOT NULL);")
	if err != nil {
		return fail(err)
	}
	_, err = tx.Exec("CREATE INDEX IF NOT EXISTS media_type ON media(media_type);")
	if err != nil {
		return fail(err)
	}
	err = tx.Commit()
	if err != nil {
		return fail(err)
	}
	return nil
}

var MediaNotFoundErr = errors.New("could not find media with given parameters")

func (manager *MediaManager) GetMediaIdByNameAndYear(name string, year int) (string, error) {
	c := colly.NewCollector()

	var mediaId string
	c.OnHTML(".ipc-metadata-list .ipc-metadata-list-summary-item:first-child", func(element *colly.HTMLElement) {
		var err error
		media, err := ParseMedia(element, false)
		if err != nil {
			slog.Error("failed to parse media", "rawMedia", element.Text, "error", err)
		}
		mediaId = media.Id
	})

	c.OnRequest(func(r *colly.Request) {
		slog.Debug("visiting new website", "url", r.URL)
	})

	url := fmt.Sprintf(IMDBSearchUrl, url2.QueryEscape(name), year, year)

	if err := c.Visit(url); err != nil {
		return "", fmt.Errorf("could search for imdb media: %w", err)
	}
	if mediaId == "" {
		return "", MediaNotFoundErr
	}
	return mediaId, nil
}

func (manager *MediaManager) ShouldMediaBeDownloaded(id string) (bool, error) {
	result, err := manager.db.Query("SELECT id FROM media WHERE id = $1;", id)
	if err != nil {
		return false, fmt.Errorf("could not check if media should be downloaded: %w", err)
	}
	return result.Next(), nil
}

func (manager *MediaManager) RefreshPopularMedia() error {
	slog.Info("refreshing popular media by scraping from imdb and updating database")
	popularMedia, err := manager.scrapePopularMedia()
	if err != nil {
		return err
	}
	slog.Info("scraped popular media", "mediaCount", len(popularMedia))
	slog.Info("replacing media within database...")
	err = manager.replaceMedia(popularMedia)
	if err != nil {
		return err
	}
	slog.Info("done with refreshing popular media")
	return nil
}

func (manager *MediaManager) replaceMedia(mediaList []*Media) error {
	tx, err := manager.db.Begin()
	fail := func(err error) error {
		return fmt.Errorf("could not replace media: %w", err)
	}
	if err != nil {
		return fail(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM media;"); err != nil {
		return fail(err)
	}
	for _, media := range mediaList {
		_, err = tx.Exec("INSERT INTO media (id, title, year, rank, media_type, url, rating, metadata_updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP);",
			media.Id, media.Title, media.Year, media.Rank, media.Type, media.Url, media.Rating)
		if err != nil {
			return fail(err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fail(err)
	}
	return nil
}

func (manager *MediaManager) scrapePopularMedia() ([]*Media, error) {
	c := colly.NewCollector()

	mediaList := make([]*Media, 0)
	c.OnHTML(".ipc-metadata-list", func(e *colly.HTMLElement) {
		e.ForEach(".cli-children", func(i int, element *colly.HTMLElement) {
			media, err := ParseMedia(element, true)
			if err != nil {
				slog.Error("failed to parse media", "rawMedia", element.Text, "error", err)
			}
			slog.Debug("parsed media", "media", media)
			mediaList = append(mediaList, media)
		})
	})

	c.OnRequest(func(r *colly.Request) {
		slog.Debug("visiting new website", "url", r.URL)
	})

	if err := c.Visit(IMDBPopularMoviesUrl); err != nil {
		return nil, fmt.Errorf("could scrape popular imdb movies: %w", err)
	}
	if err := c.Visit(IMDBPopularTVShowsUrl); err != nil {
		return nil, fmt.Errorf("could scrape popular imdb tv shows: %w", err)
	}
	return mediaList, nil
}

var idRegex = regexp.MustCompile("title/(tt\\d+)")

func ParseMedia(element *colly.HTMLElement, mustParseRank bool) (*Media, error) {
	rankText := element.ChildText(".cli-meter-title-header")
	rank, err := getFirstNumberFromText(rankText)
	if err != nil && mustParseRank {
		return nil, fmt.Errorf("failed to parse rank: %w", err)
	}

	titleElement := element.DOM.Find(".ipc-title")
	title := titleElement.Text()

	relativeUrl := titleElement.Find("a").AttrOr("href", "")
	if relativeUrl == "" {
		return nil, errors.New("could not fetch url from element")
	}
	url := element.Request.AbsoluteURL(relativeUrl)

	idMatch := idRegex.FindAllStringSubmatch(url, -1)
	if len(idMatch) == 0 || len(idMatch[0]) < 2 {
		return nil, errors.New("could not fetch id from url: " + url)
	}
	id := idMatch[0][1]

	year, err := getFirstNumberFromText(element.ChildText(".cli-title-metadata .cli-title-metadata-item:first-child"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse media year: %w", err)
	}

	rating, err := getFirstNumberFromText(element.ChildText(".cli-ratings-container .ipc-rating-star"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse media rating: %w", err)
	}

	rawMediaType := element.ChildText(".cli-title-type-data")
	mediaType := Movie
	if rawMediaType == "TV Series" {
		mediaType = TVShow
	}

	return &Media{
		Id:     id,
		Title:  title,
		Year:   int(year),
		Rank:   int(rank),
		Rating: rating,
		Type:   mediaType,
		Url:    url,
	}, nil
}

var numberRegex = regexp.MustCompile("(\\d+(\\.\\d+)?)")

func getFirstNumberFromText(text string) (float64, error) {
	match := numberRegex.FindAllStringSubmatch(text, -1)
	if len(match) == 0 || len(match[0]) == 0 {
		return -1, nil
	}
	return strconv.ParseFloat(match[0][0], 64)
}
