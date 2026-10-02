/** @type {import('next').NextConfig} */
const apiTarget = process.env.HOOKREPLAY_API_PROXY || "http://localhost:8080";

const nextConfig = {
  async rewrites() {
    return [
      // Proxy API calls to the Go server (no CORS in local/dev, and inside
      // docker-compose this points at the `api` service via HOOKREPLAY_API_PROXY).
      { source: "/v1/:path*", destination: `${apiTarget}/v1/:path*` },
    ];
  },
};

module.exports = nextConfig;
