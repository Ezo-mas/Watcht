const TMDB_BASE_URL = "https://api.themoviedb.org/3";
const DEFAULT_SEARCH_REVALIDATE = 300;
const DEFAULT_RETRIES = 2;

export type TmdbSearchResult = {
  id: number;
  mediaType: "movie" | "show";
  title: string;
  releaseYear: number | null;
  overview: string | null;
  posterPath: string | null;
};

export type TmdbSearchResponse = {
  query: string;
  page: number;
  totalPages: number;
  totalResults: number;
  results: TmdbSearchResult[];
};

type TmdbFetchOptions = {
  query?: Record<string, string | number | boolean | null | undefined>;
  revalidateSeconds?: number;
  retries?: number;
};

function getTmdbApiKey() {
  const apiKey = process.env.TMDB_API_KEY;

  if (!apiKey) {
    throw new Error("TMDB_API_KEY is required to call TMDB endpoints.");
  }

  return apiKey;
}

function buildQueryString(query: TmdbFetchOptions["query"], apiKey: string) {
  const params = new URLSearchParams();
  params.set("api_key", apiKey);

  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value === undefined || value === null || value === "") {
        continue;
      }

      params.set(key, String(value));
    }
  }

  return params;
}

async function sleep(ms: number) {
  await new Promise((resolve) => setTimeout(resolve, ms));
}

async function tmdbFetch<T>(path: string, options: TmdbFetchOptions = {}): Promise<T> {
  const apiKey = getTmdbApiKey();
  const params = buildQueryString(options.query, apiKey);
  const url = `${TMDB_BASE_URL}${path}?${params.toString()}`;
  const revalidateSeconds = options.revalidateSeconds ?? DEFAULT_SEARCH_REVALIDATE;
  const retries = options.retries ?? DEFAULT_RETRIES;

  let attempt = 0;

  while (true) {
    const response = await fetch(url, {
      headers: { Accept: "application/json" },
      next: { revalidate: revalidateSeconds },
    });

    if (response.ok) {
      return (await response.json()) as T;
    }

    const retryable = response.status === 429 || response.status >= 500;
    attempt += 1;

    if (!retryable || attempt > retries) {
      const body = await response.text().catch(() => "");
      throw new Error(`TMDB request failed (${response.status}). ${body}`.trim());
    }

    await sleep(Math.min(500 * attempt, 2000));
  }
}

type TmdbSearchApiResponse = {
  page: number;
  total_pages: number;
  total_results: number;
  results: Array<{
    id: number;
    media_type: "movie" | "tv" | string;
    title?: string | null;
    name?: string | null;
    release_date?: string | null;
    first_air_date?: string | null;
    overview?: string | null;
    poster_path?: string | null;
  }>;
};

type TmdbSearchApiItem = {
  id: number;
  media_type: "movie" | "tv";
  title?: string | null;
  name?: string | null;
  release_date?: string | null;
  first_air_date?: string | null;
  overview?: string | null;
  poster_path?: string | null;
};

function toYear(value: string | null | undefined) {
  if (!value) {
    return null;
  }

  const year = Number.parseInt(value.slice(0, 4), 10);
  return Number.isNaN(year) ? null : year;
}

function normalizeSearchResults(query: string, data: TmdbSearchApiResponse): TmdbSearchResponse {
  const results: TmdbSearchResult[] = (data.results ?? [])
    .filter((item): item is TmdbSearchApiItem => item.media_type === "movie" || item.media_type === "tv")
    .map((item): TmdbSearchResult => ({
      id: item.id,
      mediaType: item.media_type === "movie" ? "movie" : "show",
      title: item.title?.trim() || item.name?.trim() || "Untitled",
      releaseYear: toYear(item.release_date ?? item.first_air_date ?? null),
      overview: item.overview ?? null,
      posterPath: item.poster_path ?? null,
    }));

  return {
    query,
    page: data.page,
    totalPages: data.total_pages,
    totalResults: data.total_results,
    results,
  };
}

export async function searchMultiTmdb(query: string, page = 1): Promise<TmdbSearchResponse> {
  const data = await tmdbFetch<TmdbSearchApiResponse>("/search/multi", {
    query: {
      query,
      page,
      include_adult: false,
      language: "en-US",
    },
  });

  return normalizeSearchResults(query, data);
}
