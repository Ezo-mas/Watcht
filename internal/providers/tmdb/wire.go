package tmdb

import "time"

// The wire types decode the parts of TMDB's JSON that watcht reads. They never
// leave this package: each method converts them to the exported structs. A
// missing or null field decodes to its zero value, which the exported structs
// use for "unknown": 0 for numbers, "" for strings and dates.

// wireExternalIDs is the external_ids object that append_to_response adds.
type wireExternalIDs struct {
	IMDbID string `json:"imdb_id"`
	TVDBID int    `json:"tvdb_id"`
}

type wireGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type wireSearch struct {
	Page         int              `json:"page"`
	TotalPages   int              `json:"total_pages"`
	TotalResults int              `json:"total_results"`
	Results      []wireSearchItem `json:"results"`
}

// wireSearchItem is a movie, show, or person in search or find results. TMDB
// fills title and release_date for movies, and name and first_air_date for
// shows.
type wireSearchItem struct {
	ID            int     `json:"id"`
	MediaType     string  `json:"media_type"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseDate   string  `json:"release_date"`
	Name          string  `json:"name"`
	OriginalName  string  `json:"original_name"`
	FirstAirDate  string  `json:"first_air_date"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	VoteAverage   float64 `json:"vote_average"`
}

type wireFind struct {
	MovieResults     []wireSearchItem `json:"movie_results"`
	TVResults        []wireSearchItem `json:"tv_results"`
	TVEpisodeResults []wireEpisode    `json:"tv_episode_results"`
}

type wireMovie struct {
	ID               int             `json:"id"`
	Title            string          `json:"title"`
	OriginalTitle    string          `json:"original_title"`
	OriginalLanguage string          `json:"original_language"`
	Overview         string          `json:"overview"`
	Tagline          string          `json:"tagline"`
	ReleaseDate      string          `json:"release_date"`
	Status           string          `json:"status"`
	PosterPath       string          `json:"poster_path"`
	BackdropPath     string          `json:"backdrop_path"`
	Runtime          int             `json:"runtime"`
	Genres           []wireGenre     `json:"genres"`
	VoteAverage      float64         `json:"vote_average"`
	VoteCount        int             `json:"vote_count"`
	ExternalIDs      wireExternalIDs `json:"external_ids"`
}

type wireShow struct {
	ID               int                 `json:"id"`
	Name             string              `json:"name"`
	OriginalName     string              `json:"original_name"`
	OriginalLanguage string              `json:"original_language"`
	Overview         string              `json:"overview"`
	Tagline          string              `json:"tagline"`
	FirstAirDate     string              `json:"first_air_date"`
	LastAirDate      string              `json:"last_air_date"`
	Status           string              `json:"status"`
	PosterPath       string              `json:"poster_path"`
	BackdropPath     string              `json:"backdrop_path"`
	InProduction     bool                `json:"in_production"`
	NumberOfSeasons  int                 `json:"number_of_seasons"`
	NumberOfEpisodes int                 `json:"number_of_episodes"`
	Genres           []wireGenre         `json:"genres"`
	Seasons          []wireSeasonSummary `json:"seasons"`
	VoteAverage      float64             `json:"vote_average"`
	VoteCount        int                 `json:"vote_count"`
	ExternalIDs      wireExternalIDs     `json:"external_ids"`
}

type wireSeasonSummary struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"season_number"`
	EpisodeCount int    `json:"episode_count"`
	Name         string `json:"name"`
	AirDate      string `json:"air_date"`
	PosterPath   string `json:"poster_path"`
}

// wireSeason has no season number: the caller asked for one, and the
// conversion uses that.
type wireSeason struct {
	ID         int           `json:"id"`
	Name       string        `json:"name"`
	Overview   string        `json:"overview"`
	AirDate    string        `json:"air_date"`
	PosterPath string        `json:"poster_path"`
	Episodes   []wireEpisode `json:"episodes"`
}

// wireEpisode is an episode from a season, an episode lookup, or find results.
// Only the episode lookup asks for external IDs, and find results carry the
// show ID that the other two get from the request.
type wireEpisode struct {
	ID            int             `json:"id"`
	ShowID        int             `json:"show_id"`
	SeasonNumber  int             `json:"season_number"`
	EpisodeNumber int             `json:"episode_number"`
	Name          string          `json:"name"`
	Overview      string          `json:"overview"`
	AirDate       string          `json:"air_date"`
	StillPath     string          `json:"still_path"`
	Runtime       int             `json:"runtime"`
	VoteAverage   float64         `json:"vote_average"`
	ExternalIDs   wireExternalIDs `json:"external_ids"`
}

// searchResults keeps the movies and shows from a multi search. TMDB's
// multi search also returns people, which watcht doesn't use.
func searchResults(w wireSearch) SearchResults {
	out := SearchResults{
		Page:         w.Page,
		TotalPages:   w.TotalPages,
		TotalResults: w.TotalResults,
		Results:      make([]SearchResult, 0, len(w.Results)),
	}
	for _, item := range w.Results {
		if item.MediaType == "movie" || item.MediaType == "tv" {
			out.Results = append(out.Results, searchResult(item.MediaType, item))
		}
	}
	return out
}

// searchResult converts a movie or a show. mediaType says which, because
// TMDB puts a movie's title in title and a show's in name.
func searchResult(mediaType string, w wireSearchItem) SearchResult {
	r := SearchResult{
		MediaType:   mediaType,
		TMDBID:      w.ID,
		Overview:    w.Overview,
		PosterPath:  w.PosterPath,
		VoteAverage: w.VoteAverage,
	}
	if mediaType == "movie" {
		r.Title, r.OriginalTitle, r.Date = w.Title, w.OriginalTitle, validDate(w.ReleaseDate)
	} else {
		r.Title, r.OriginalTitle, r.Date = w.Name, w.OriginalName, validDate(w.FirstAirDate)
	}
	return r
}

// findResults splits a find response into its lists. A find episode's show is
// the show_id that TMDB sends with it.
func findResults(w wireFind) FindResults {
	out := FindResults{
		Movies:   make([]SearchResult, 0, len(w.MovieResults)),
		Shows:    make([]SearchResult, 0, len(w.TVResults)),
		Episodes: make([]Episode, 0, len(w.TVEpisodeResults)),
	}
	for _, m := range w.MovieResults {
		out.Movies = append(out.Movies, searchResult("movie", m))
	}
	for _, s := range w.TVResults {
		out.Shows = append(out.Shows, searchResult("tv", s))
	}
	for _, e := range w.TVEpisodeResults {
		out.Episodes = append(out.Episodes, episodeFromWire(e, e.ShowID, e.SeasonNumber))
	}
	return out
}

func movieFromWire(w wireMovie) Movie {
	return Movie{
		TMDBID:           w.ID,
		IMDbID:           w.ExternalIDs.IMDbID,
		Title:            w.Title,
		OriginalTitle:    w.OriginalTitle,
		OriginalLanguage: w.OriginalLanguage,
		Overview:         w.Overview,
		Tagline:          w.Tagline,
		ReleaseDate:      validDate(w.ReleaseDate),
		Status:           w.Status,
		PosterPath:       w.PosterPath,
		BackdropPath:     w.BackdropPath,
		Runtime:          w.Runtime,
		Genres:           genres(w.Genres),
		VoteAverage:      w.VoteAverage,
		VoteCount:        w.VoteCount,
	}
}

func showFromWire(w wireShow) Show {
	return Show{
		TMDBID:           w.ID,
		IMDbID:           w.ExternalIDs.IMDbID,
		TVDBID:           w.ExternalIDs.TVDBID,
		Name:             w.Name,
		OriginalName:     w.OriginalName,
		OriginalLanguage: w.OriginalLanguage,
		Overview:         w.Overview,
		Tagline:          w.Tagline,
		FirstAirDate:     validDate(w.FirstAirDate),
		LastAirDate:      validDate(w.LastAirDate),
		Status:           w.Status,
		PosterPath:       w.PosterPath,
		BackdropPath:     w.BackdropPath,
		InProduction:     w.InProduction,
		Ended:            w.Status == "Ended" || w.Status == "Canceled",
		NumberOfSeasons:  w.NumberOfSeasons,
		NumberOfEpisodes: w.NumberOfEpisodes,
		Genres:           genres(w.Genres),
		Seasons:          seasonSummaries(w.Seasons),
		VoteAverage:      w.VoteAverage,
		VoteCount:        w.VoteCount,
	}
}

func seasonSummaries(ws []wireSeasonSummary) []SeasonSummary {
	out := make([]SeasonSummary, 0, len(ws))
	for _, s := range ws {
		out = append(out, SeasonSummary{
			TMDBID:       s.ID,
			SeasonNumber: s.SeasonNumber,
			EpisodeCount: s.EpisodeCount,
			Name:         s.Name,
			AirDate:      validDate(s.AirDate),
			PosterPath:   s.PosterPath,
		})
	}
	return out
}

// seasonFromWire takes the show and season from the request, not the
// response, so every episode carries the show ID even when TMDB omits it.
func seasonFromWire(w wireSeason, showID, seasonNumber int) Season {
	s := Season{
		TMDBID:       w.ID,
		ShowTMDBID:   showID,
		SeasonNumber: seasonNumber,
		Name:         w.Name,
		Overview:     w.Overview,
		AirDate:      validDate(w.AirDate),
		PosterPath:   w.PosterPath,
		Episodes:     make([]Episode, 0, len(w.Episodes)),
	}
	for _, e := range w.Episodes {
		s.Episodes = append(s.Episodes, episodeFromWire(e, showID, seasonNumber))
	}
	return s
}

func episodeFromWire(w wireEpisode, showID, seasonNumber int) Episode {
	return Episode{
		TMDBID:        w.ID,
		ShowTMDBID:    showID,
		SeasonNumber:  seasonNumber,
		EpisodeNumber: w.EpisodeNumber,
		Name:          w.Name,
		Overview:      w.Overview,
		AirDate:       validDate(w.AirDate),
		StillPath:     w.StillPath,
		Runtime:       w.Runtime,
		VoteAverage:   w.VoteAverage,
		IMDbID:        w.ExternalIDs.IMDbID,
		TVDBID:        w.ExternalIDs.TVDBID,
	}
}

func genres(ws []wireGenre) []Genre {
	out := make([]Genre, 0, len(ws))
	for _, g := range ws {
		out = append(out, Genre(g))
	}
	return out
}

// validDate keeps s only if it's a real day in YYYY-MM-DD form. TMDB sends
// null or "" for an unknown date, and anything else unparsable is treated the
// same way.
func validDate(s string) string {
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return ""
	}
	return s
}
