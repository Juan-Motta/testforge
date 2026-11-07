import { NextResponse, type NextRequest } from "next/server";

const SESSION_COOKIE = "tf_session";

/**
 * Optimistic route protection: without a session cookie there is no point in
 * rendering a workspace. The workspace layout still validates the session with
 * the API and redirects on 401.
 */
export function proxy(request: NextRequest) {
  if (!request.cookies.has(SESSION_COOKIE)) {
    return NextResponse.redirect(new URL("/login", request.url));
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/", "/w/:path*"],
};
