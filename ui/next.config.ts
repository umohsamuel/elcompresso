import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // The repo root has its own package.json/lockfile, so Next would otherwise
  // infer it as the workspace root. Pin it to this app.
  turbopack: {
    root: __dirname,
  },
  experimental: {
    turbopackFileSystemCacheForDev: false,
    preloadEntriesOnStart: false,
  },
};

export default nextConfig;
