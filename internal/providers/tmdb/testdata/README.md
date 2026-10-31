# TMDB fixtures

Recorded from `https://api.themoviedb.org/3` on 2026-10-31 (en-US, the API
default). Each file is one response body, named after its request:

| File | Request |
| --- | --- |
| `search_multi.json` | `GET /search/multi?query=Matrix` |
| `search_multi_person.json` | `GET /search/multi?query=Harrison Ford` (a person among movies and shows) |
| `search_multi_empty.json` | `GET /search/multi?query=zzqxjvnonexistenttitle` |
| `movie_603.json` | `GET /movie/603?append_to_response=external_ids` |
| `tv_1396.json` | `GET /tv/1396?append_to_response=external_ids` (ended show) |
| `tv_94997.json` | `GET /tv/94997?append_to_response=external_ids` (returning show) |
| `tv_1437.json` | `GET /tv/1437?append_to_response=external_ids` (canceled show) |
| `tv_1396_season_1.json` | `GET /tv/1396/season/1` |
| `tv_1396_season_0.json` | `GET /tv/1396/season/0` (specials) |
| `tv_1396_season_1_episode_1.json` | `GET /tv/1396/season/1/episode/1?append_to_response=external_ids` |
| `find_imdb_tt0903747.json` | `GET /find/tt0903747?external_source=imdb_id` (a show) |
| `find_imdb_tt0133093.json` | `GET /find/tt0133093?external_source=imdb_id` (a movie) |
| `find_tvdb_81189.json` | `GET /find/81189?external_source=tvdb_id` |
| `find_tvdb_349232.json` | `GET /find/349232?external_source=tvdb_id` (an episode) |
| `find_none.json` | `GET /find/tt0000000?external_source=imdb_id` (no match) |
| `movie_not_found.json` | `GET /movie/999999999` (status 404) |

Trimmed: search results keep the first 5 (`total_results` and
`total_pages` are as recorded); seasons and the episode drop `crew` and
`guest_stars`. Nothing else was edited. Refresh a fixture by re-recording
it, never by hand-editing it.
