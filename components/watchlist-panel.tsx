"use client";

import { FormEvent, useCallback, useMemo, useState } from "react";

export type WatchlistPanelItem = {
  id: number;
  itemType: "movie" | "show";
  movieId: number | null;
  showId: number | null;
  listedAt: string | null;
  title: string | null;
};

type TmdbSearchResult = {
  id: number;
  mediaType: "movie" | "show";
  title: string;
  releaseYear: number | null;
  overview: string | null;
  posterPath: string | null;
};

type TmdbSearchResponse = {
  results: TmdbSearchResult[];
};

type RequestState = "idle" | "loading";

type WatchlistPanelProps = {
  initialItems: WatchlistPanelItem[];
};

async function parseError(response: Response) {
  const fallback = `Request failed with status ${response.status}`;
  const body = await response.json().catch(() => null);

  if (!body || typeof body !== "object") {
    return fallback;
  }

  if (typeof body.error === "string") {
    return body.error;
  }

  return fallback;
}

export default function WatchlistPanel({ initialItems }: WatchlistPanelProps) {
  const [items, setItems] = useState<WatchlistPanelItem[]>(initialItems);
  const [error, setError] = useState<string | null>(null);
  const [requestState, setRequestState] = useState<RequestState>("idle");
  const [addType, setAddType] = useState<"movie" | "show">("movie");
  const [titleInput, setTitleInput] = useState("");
  const [yearInput, setYearInput] = useState("");
  const [showEpisodeInputs, setShowEpisodeInputs] = useState<Record<number, { season: string; episode: string }>>({});
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<TmdbSearchResult[]>([]);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [searchState, setSearchState] = useState<RequestState>("idle");

  const loading = requestState === "loading";
  const searching = searchState === "loading";

  const fetchItems = useCallback(async () => {
    try {
      const response = await fetch("/api/watchlist", { cache: "no-store" });

      if (!response.ok) {
        throw new Error(await parseError(response));
      }

      const data = (await response.json()) as { items?: WatchlistPanelItem[] };
      setItems(Array.isArray(data.items) ? data.items : []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load watchlist");
    }
  }, []);

  const onAdd = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();

      const cleanedTitle = titleInput.trim();
      if (!cleanedTitle) {
        setError("Title is required");
        return;
      }

      setRequestState("loading");
      setError(null);

      const maybeYear = Number.parseInt(yearInput, 10);
      const payload = {
        itemType: addType,
        title: cleanedTitle,
        releaseYear: Number.isNaN(maybeYear) ? undefined : maybeYear,
      };

      try {
        const response = await fetch("/api/watchlist", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });

        if (!response.ok) {
          throw new Error(await parseError(response));
        }

        setTitleInput("");
        setYearInput("");
        await fetchItems();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unable to add watchlist item");
      } finally {
        setRequestState("idle");
      }
    },
    [addType, titleInput, yearInput, fetchItems]
  );

  const onSearch = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();

      const cleanedQuery = searchQuery.trim();
      if (!cleanedQuery) {
        setSearchError("Search query is required");
        return;
      }

      setSearchState("loading");
      setSearchError(null);

      try {
        const response = await fetch(`/api/tmdb/search?q=${encodeURIComponent(cleanedQuery)}`, {
          cache: "no-store",
        });

        if (!response.ok) {
          throw new Error(await parseError(response));
        }

        const data = (await response.json()) as TmdbSearchResponse;
        setSearchResults(Array.isArray(data.results) ? data.results : []);
      } catch (err) {
        setSearchError(err instanceof Error ? err.message : "Unable to search TMDB");
      } finally {
        setSearchState("idle");
      }
    },
    [searchQuery]
  );

  const onAddFromSearch = useCallback(
    async (result: TmdbSearchResult) => {
      setRequestState("loading");
      setError(null);

      try {
        const response = await fetch("/api/watchlist", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            itemType: result.mediaType,
            tmdbId: result.id,
            title: result.title,
            releaseYear: result.releaseYear ?? undefined,
          }),
        });

        if (!response.ok) {
          throw new Error(await parseError(response));
        }

        await fetchItems();
        setSearchResults((previous) =>
          previous.filter((item) => !(item.id === result.id && item.mediaType === result.mediaType))
        );
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unable to add watchlist item");
      } finally {
        setRequestState("idle");
      }
    },
    [fetchItems]
  );

  const onRemove = useCallback(
    async (id: number) => {
      setRequestState("loading");
      setError(null);

      try {
        const response = await fetch("/api/watchlist", {
          method: "DELETE",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ id }),
        });

        if (!response.ok) {
          throw new Error(await parseError(response));
        }

        await fetchItems();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unable to remove watchlist item");
      } finally {
        setRequestState("idle");
      }
    },
    [fetchItems]
  );

  const onMarkWatched = useCallback(
    async (item: WatchlistPanelItem) => {
      setRequestState("loading");
      setError(null);

      const payload: {
        watchlistItemId: number;
        removeFromWatchlist: boolean;
        seasonNumber?: number;
        episodeNumber?: number;
      } = {
        watchlistItemId: item.id,
        removeFromWatchlist: item.itemType === "movie",
      };

      if (item.itemType === "show") {
        const showInput = showEpisodeInputs[item.id] ?? { season: "", episode: "" };
        const season = Number.parseInt(showInput.season, 10);
        const episode = Number.parseInt(showInput.episode, 10);

        if (Number.isNaN(season) || season < 1 || Number.isNaN(episode) || episode < 1) {
          setError("Provide valid season and episode numbers for show progress.");
          setRequestState("idle");
          return;
        }

        payload.seasonNumber = season;
        payload.episodeNumber = episode;
      }

      try {
        const response = await fetch("/api/watch-events", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });

        if (!response.ok) {
          throw new Error(await parseError(response));
        }

        if (item.itemType === "show") {
          setShowEpisodeInputs((previous) => ({
            ...previous,
            [item.id]: { season: "", episode: "" },
          }));
        }

        await fetchItems();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unable to mark watched");
      } finally {
        setRequestState("idle");
      }
    },
    [fetchItems, showEpisodeInputs]
  );

  const movieCount = useMemo(() => items.filter((item) => item.itemType === "movie").length, [items]);
  const showCount = useMemo(() => items.filter((item) => item.itemType === "show").length, [items]);

  return (
    <section className="card rounded-xl p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-semibold text-white">Watchlist</h2>
        <p className="text-xs uppercase tracking-wide text-slate-400">
          {items.length} total ({movieCount} movies, {showCount} shows)
        </p>
      </div>

      <form onSubmit={onAdd} className="mt-4 grid gap-3 sm:grid-cols-[auto_1fr_auto_auto]">
        <select
          value={addType}
          onChange={(event) => setAddType(event.target.value === "show" ? "show" : "movie")}
          className="rounded-md border border-white/20 bg-slate-900/80 px-3 py-2 text-sm text-slate-100"
          disabled={loading}
        >
          <option value="movie">Movie</option>
          <option value="show">Show</option>
        </select>
        <input
          value={titleInput}
          onChange={(event) => setTitleInput(event.target.value)}
          placeholder={addType === "movie" ? "Movie title" : "Show title"}
          className="rounded-md border border-white/20 bg-slate-900/80 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-500"
          disabled={loading}
        />
        <input
          value={yearInput}
          onChange={(event) => setYearInput(event.target.value)}
          placeholder="Year"
          className="w-24 rounded-md border border-white/20 bg-slate-900/80 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-500"
          disabled={loading}
          inputMode="numeric"
        />
        <button
          type="submit"
          className="rounded-md bg-amber-500 px-4 py-2 text-sm font-medium text-slate-950 transition hover:bg-amber-400 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={loading}
        >
          Add
        </button>
      </form>

      <form onSubmit={onSearch} className="mt-4 grid gap-3 sm:grid-cols-[1fr_auto]">
        <input
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.target.value)}
          placeholder="Search TMDB for movies or shows"
          className="rounded-md border border-white/20 bg-slate-900/80 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-500"
          disabled={loading || searching}
        />
        <button
          type="submit"
          className="rounded-md border border-white/20 px-4 py-2 text-sm text-slate-100 transition hover:border-white/40 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={loading || searching}
        >
          {searching ? "Searching..." : "Search"}
        </button>
      </form>

      {searchError ? <p className="mt-3 text-sm text-rose-300">{searchError}</p> : null}

      {searchResults.length > 0 ? (
        <ul className="mt-3 space-y-2">
          {searchResults.map((result) => (
            <li key={`${result.mediaType}-${result.id}`} className="rounded-md border border-white/10 bg-slate-900/40 px-3 py-2">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-medium text-white">{result.title}</p>
                  <p className="text-xs uppercase tracking-wide text-slate-400">
                    {result.mediaType} {result.releaseYear ? `- ${result.releaseYear}` : ""}
                  </p>
                  {result.overview ? <p className="mt-1 text-xs text-slate-400">{result.overview}</p> : null}
                </div>
                <button
                  type="button"
                  onClick={() => void onAddFromSearch(result)}
                  disabled={loading}
                  className="rounded-md bg-amber-500 px-3 py-1 text-xs font-medium text-slate-950 transition hover:bg-amber-400 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  Add
                </button>
              </div>
            </li>
          ))}
        </ul>
      ) : searchQuery.trim() ? (
        <p className="mt-3 text-sm text-slate-400">No TMDB results yet. Try another title.</p>
      ) : null}

      {error ? <p className="mt-3 text-sm text-rose-300">{error}</p> : null}

      <ul className="mt-4 space-y-2">
        {items.map((item) => (
          <li key={item.id} className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-white/10 bg-slate-900/50 px-3 py-2">
            <div>
              <p className="text-sm font-medium text-white">{item.title ?? `Untitled ${item.itemType}`}</p>
              <p className="text-xs uppercase tracking-wide text-slate-400">
                {item.itemType} #{item.itemType === "movie" ? item.movieId : item.showId}
              </p>
            </div>
            <div className="flex items-center gap-2">
              {item.itemType === "show" ? (
                <>
                  <input
                    value={showEpisodeInputs[item.id]?.season ?? ""}
                    onChange={(event) =>
                      setShowEpisodeInputs((previous) => ({
                        ...previous,
                        [item.id]: {
                          season: event.target.value,
                          episode: previous[item.id]?.episode ?? "",
                        },
                      }))
                    }
                    placeholder="S"
                    inputMode="numeric"
                    disabled={loading}
                    className="w-14 rounded-md border border-white/20 bg-slate-900/80 px-2 py-1 text-xs text-slate-100 placeholder:text-slate-500"
                  />
                  <input
                    value={showEpisodeInputs[item.id]?.episode ?? ""}
                    onChange={(event) =>
                      setShowEpisodeInputs((previous) => ({
                        ...previous,
                        [item.id]: {
                          season: previous[item.id]?.season ?? "",
                          episode: event.target.value,
                        },
                      }))
                    }
                    placeholder="E"
                    inputMode="numeric"
                    disabled={loading}
                    className="w-14 rounded-md border border-white/20 bg-slate-900/80 px-2 py-1 text-xs text-slate-100 placeholder:text-slate-500"
                  />
                </>
              ) : null}
              <button
                type="button"
                onClick={() => void onRemove(item.id)}
                disabled={loading}
                className="rounded-md border border-white/20 px-3 py-1 text-xs text-slate-100 transition hover:border-white/40 disabled:cursor-not-allowed disabled:opacity-60"
              >
                Remove
              </button>
              <button
                type="button"
                onClick={() => void onMarkWatched(item)}
                disabled={loading}
                className="rounded-md bg-emerald-500/90 px-3 py-1 text-xs font-medium text-slate-950 transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {item.itemType === "movie" ? "Mark watched" : "Log episode"}
              </button>
            </div>
          </li>
        ))}
      </ul>

      {items.length === 0 ? (
        <p className="mt-4 text-sm text-slate-300">
          Your watchlist is empty. Add a movie or show above to start tracking.
        </p>
      ) : null}
    </section>
  );
}