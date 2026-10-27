// Types for /api/v1 responses. They mirror the Go handlers in
// internal/httpapi; change both together.

/** The standard error body: {"error": {"code": "...", "message": "..."}}. */
export interface ErrorBody {
	error: { code: string; message: string };
}

/** A signed-in user (internal/httpapi userJSON). */
export interface User {
	id: number;
	username: string;
	is_admin: boolean;
}

/** GET /api/v1/auth/session. */
export interface SessionStatus {
	setup_required: boolean;
	user: User | null;
}

/** An account in admin user management (internal/httpapi accountJSON). */
export interface Account extends User {
	/** UTC, RFC 3339. */
	created_at: string;
}

/** An API token's description; the secret is only in CreatedToken. */
export interface APIToken {
	id: number;
	name: string;
	/** UTC, RFC 3339. */
	created_at: string;
}

/** POST /api/v1/tokens: the secret is shown this once and never again. */
export interface CreatedToken {
	token: APIToken;
	secret: string;
}
