"use client";

import { FormEvent, useCallback, useMemo, useState } from "react";

export type RatingsSummary = {
  total: number;
  average: number | null;
  distribution: Array<{ rating: number; count: number }>;
};

type RatingsPanelProps = {
  initialSummary: RatingsSummary;
};

type RequestState = "idle" | "loading";

type StarFill = 0 | 50 | 100;

function StarIcon({ fill, idSuffix }: { fill: StarFill; idSuffix: string }) {
  const gradientId = `star-gradient-${idSuffix}`;

  return (
    <svg viewBox="0 0 24 24" className="h-6 w-6">
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="0">
          <stop offset={`${fill}%`} stopColor="#f59e0b" />
          <stop offset={`${fill}%`} stopColor="#1f2937" />
        </linearGradient>
      </defs>
      <path
        d="M12 2l2.95 6.32 6.98.6-5.28 4.55 1.6 6.78L12 16.9 5.75 20.25l1.6-6.78L2.07 8.92l6.98-.6L12 2z"
        fill={`url(#${gradientId})`}
        stroke="#f59e0b"
        strokeWidth="1"
      />
    </svg>
  );
}

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

export default function RatingsPanel({ initialSummary }: RatingsPanelProps) {
  const [summary, setSummary] = useState<RatingsSummary>(initialSummary);
  const [requestState, setRequestState] = useState<RequestState>("idle");
  const [error, setError] = useState<string | null>(null);
  const [itemType, setItemType] = useState<"movie" | "show">("movie");
  const [title, setTitle] = useState("");
  const [year, setYear] = useState("");
  const [ratingValue, setRatingValue] = useState(4);

  const loading = requestState === "loading";

  const starDistribution = useMemo(
    () => summary.distribution.map((item) => ({ label: (item.rating / 2).toFixed(1), count: item.count })),
    [summary]
  );
  const maxCount = useMemo(() => Math.max(1, ...summary.distribution.map((item) => item.count)), [summary]);

  const refreshSummary = useCallback(async () => {
    const response = await fetch("/api/ratings", { cache: "no-store" });

    if (!response.ok) {
      throw new Error(await parseError(response));
    }

    const data = (await response.json()) as RatingsSummary;
    setSummary(data);
  }, []);

  const onSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();

      const cleanedTitle = title.trim();
      if (!cleanedTitle) {
        setError("Title is required for rating.");
        return;
      }

      const parsedRating = Math.round(ratingValue * 2);
      if (parsedRating < 1 || parsedRating > 10) {
        setError("Rating must be between 0.5 and 5 stars.");
        return;
      }

      setRequestState("loading");
      setError(null);

      const maybeYear = Number.parseInt(year, 10);
      const payload = {
        itemType,
        title: cleanedTitle,
        releaseYear: Number.isNaN(maybeYear) ? undefined : maybeYear,
        ratingValue: parsedRating,
      };

      try {
        const response = await fetch("/api/ratings", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });

        if (!response.ok) {
          throw new Error(await parseError(response));
        }

        setTitle("");
        setYear("");
        await refreshSummary();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unable to save rating");
      } finally {
        setRequestState("idle");
      }
    },
    [itemType, ratingValue, title, year, refreshSummary]
  );

  const averageStars = summary.average ? summary.average / 2 : null;

  return (
    <section className="card rounded-xl p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-white">Ratings</h2>
          <p className="text-xs uppercase tracking-wide text-slate-400">
            {summary.total} ratings ·
            {averageStars ? ` ${averageStars.toFixed(2)} / 5 (${summary.average?.toFixed(2)} / 10)` : " No average yet"}
          </p>
        </div>
        <div className="rounded-full border border-white/10 bg-slate-900/60 px-3 py-1 text-xs text-slate-200">
          Your rating distribution
        </div>
      </div>

      <div className="mt-4 grid gap-2">
        {starDistribution.map((item) => (
          <div key={item.label} className="flex items-center gap-3">
            <span className="w-8 text-xs text-slate-400">{item.label}</span>
            <div className="h-2 flex-1 rounded-full bg-slate-900/60">
              <div
                className="h-2 rounded-full bg-amber-500/80"
                style={{ width: `${Math.round((item.count / maxCount) * 100)}%` }}
              />
            </div>
            <span className="w-8 text-right text-xs text-slate-400">{item.count}</span>
          </div>
        ))}
      </div>

      <form onSubmit={onSubmit} className="mt-5 grid gap-3 sm:grid-cols-[auto_1fr_auto]">
        <select
          value={itemType}
          onChange={(event) => setItemType(event.target.value === "show" ? "show" : "movie")}
          className="rounded-md border border-white/20 bg-slate-900/80 px-3 py-2 text-sm text-slate-100"
          disabled={loading}
        >
          <option value="movie">Movie</option>
          <option value="show">Show</option>
        </select>
        <input
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="Title"
          className="rounded-md border border-white/20 bg-slate-900/80 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-500"
          disabled={loading}
        />
        <input
          value={year}
          onChange={(event) => setYear(event.target.value)}
          placeholder="Year"
          className="w-24 rounded-md border border-white/20 bg-slate-900/80 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-500"
          disabled={loading}
          inputMode="numeric"
        />
        <div className="flex items-center gap-2">
          <div className="flex items-center">
            {Array.from({ length: 5 }, (_, index) => {
              const starValue = index + 1;
              const fill: StarFill =
                ratingValue >= starValue ? 100 : ratingValue >= starValue - 0.5 ? 50 : 0;

              return (
                <div key={starValue} className="relative h-6 w-6">
                  <button
                    type="button"
                    className="absolute left-0 top-0 h-6 w-3"
                    onClick={() => setRatingValue(starValue - 0.5)}
                    disabled={loading}
                    aria-label={`${starValue - 0.5} stars`}
                  />
                  <button
                    type="button"
                    className="absolute right-0 top-0 h-6 w-3"
                    onClick={() => setRatingValue(starValue)}
                    disabled={loading}
                    aria-label={`${starValue} stars`}
                  />
                  <StarIcon fill={fill} idSuffix={`input-${starValue}-${fill}`} />
                </div>
              );
            })}
          </div>
          <span className="text-xs text-slate-400">{ratingValue.toFixed(1)} / 5</span>
        </div>
        <button
          type="submit"
          className="rounded-md bg-amber-500 px-4 py-2 text-sm font-medium text-slate-950 transition hover:bg-amber-400 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={loading}
        >
          {loading ? "Saving..." : "Save rating"}
        </button>
      </form>

      {error ? <p className="mt-3 text-sm text-rose-300">{error}</p> : null}
    </section>
  );
}
