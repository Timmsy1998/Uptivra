import { NextRequest } from "next/server";
export const runtime = "nodejs";
export const dynamic = "force-dynamic";
// Forward a fixed set of headers to a fixed backend; never accept a destination from the request.
async function forward(request: NextRequest) {
  const base = process.env.API_ORIGIN;
  if (!base)
    return Response.json(
      { error: "API_ORIGIN is not configured" },
      { status: 503 },
    );
  const url = new URL(request.nextUrl.pathname + request.nextUrl.search, base);
  const headers = new Headers();
  for (const name of ["content-type", "cookie", "authorization", "origin"]) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  if (process.env.VERCEL_AUTOMATION_BYPASS_SECRET)
    headers.set(
      "x-vercel-protection-bypass",
      process.env.VERCEL_AUTOMATION_BYPASS_SECRET,
    );
  try {
    const body =
      request.method === "GET" || request.method === "HEAD"
        ? undefined
        : await request.arrayBuffer();
    if (body && body.byteLength > 32768)
      return Response.json({ error: "request too large" }, { status: 413 });
    const upstream = await fetch(url, {
      method: request.method,
      headers,
      body,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(15000),
    });
    const responseHeaders = new Headers({ "Cache-Control": "no-store" });
    for (const name of [
      "content-type",
      "set-cookie",
      "x-content-type-options",
    ]) {
      const value = upstream.headers.get(name);
      if (value) responseHeaders.set(name, value);
    }
    return new Response(upstream.body, {
      status: upstream.status,
      headers: responseHeaders,
    });
  } catch {
    return Response.json(
      { error: "monitoring service unavailable" },
      { status: 503 },
    );
  }
}
export { forward as GET, forward as POST, forward as PUT, forward as DELETE };
