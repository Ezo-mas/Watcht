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
