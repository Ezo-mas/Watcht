import type { DefaultSession } from "next-auth";

declare module "next-auth" {
  interface User {
    appUserId?: string;
    appHandle?: string;
  }

  interface Session {
    user: DefaultSession["user"] & {
      appUserId?: string;
      handle?: string;
      displayName?: string;
      isPrivate?: boolean;
      joinedAt?: string;
    };
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    appUserId?: string;
    appHandle?: string;
  }
}