import createClient from "openapi-fetch";
import type { paths } from "./schema";

// Same-origin: the built SPA is served by the Go binary itself
// (go:embed), so no CORS handling is needed. In dev, Vite's proxy (see
// vite.config.ts) forwards /api to the Go server on :8080. The schema's
// path keys are relative to the API's /api/v1 base path (Swagger's
// basePath isn't baked into each key), so it's set explicitly here.
export const api = createClient<paths>({ baseUrl: "/api/v1" });
