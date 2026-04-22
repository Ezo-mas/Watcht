type RequiredEnv = {
  NEXTAUTH_URL: string;
  NEXTAUTH_SECRET: string;
  GOOGLE_CLIENT_ID: string;
  GOOGLE_CLIENT_SECRET: string;
  GITHUB_ID: string;
  GITHUB_SECRET: string;
  DATABASE_URL: string;
  REDIS_URL: string;
  TMDB_API_KEY: string;
};

export function readRequiredEnv(): RequiredEnv {
  const {
    NEXTAUTH_URL,
    NEXTAUTH_SECRET,
    GOOGLE_CLIENT_ID,
    GOOGLE_CLIENT_SECRET,
    GITHUB_ID,
    GITHUB_SECRET,
    DATABASE_URL,
    REDIS_URL,
    TMDB_API_KEY,
  } = process.env;

  if (
    !NEXTAUTH_URL ||
    !NEXTAUTH_SECRET ||
    !GOOGLE_CLIENT_ID ||
    !GOOGLE_CLIENT_SECRET ||
    !GITHUB_ID ||
    !GITHUB_SECRET ||
    !DATABASE_URL ||
    !REDIS_URL ||
    !TMDB_API_KEY
  ) {
    throw new Error("Missing required environment variables. Copy .env.example to .env.local and fill values.");
  }

  return {
    NEXTAUTH_URL,
    NEXTAUTH_SECRET,
    GOOGLE_CLIENT_ID,
    GOOGLE_CLIENT_SECRET,
    GITHUB_ID,
    GITHUB_SECRET,
    DATABASE_URL,
    REDIS_URL,
    TMDB_API_KEY,
  };
}
