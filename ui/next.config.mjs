/** @type {import('next').NextConfig} */
const nextConfig = {
  // output: "export", // Commented out to enable rewrites in dev mode. Uncomment for production static builds.
  async rewrites() {
    return [
      {
        source: "/v1/:path*",
        destination: "http://localhost:8080/v1/:path*",
      },
    ]
  },
  typescript: {
    ignoreBuildErrors: true,
  },
  images: {
    unoptimized: true,
  },
}

export default nextConfig