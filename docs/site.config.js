export default {
  name: "den",
  description: "Secure sandbox runtime for AI agents",

  navLinks: [
    { label: "Docs", href: "#introduction" },
    { label: "API", href: "#rest-api" },
    { label: "MCP", href: "#mcp" },
    { label: "GitHub", href: "https://github.com/us/den", external: true },
  ],

  sidebar: [
    {
      title: "Getting Started",
      children: [
        { title: "Introduction", slug: "introduction" },
        { title: "Installation", slug: "installation" },
        { title: "Quick Start", slug: "quick-start" },
      ],
    },
    {
      title: "Core Concepts",
      children: [
        { title: "Architecture", slug: "architecture" },
        { title: "Configuration", slug: "configuration" },
      ],
    },
    {
      title: "API",
      children: [
        { title: "REST API", slug: "rest-api" },
        { title: "CLI Commands", slug: "cli" },
        { title: "SDKs", slug: "sdks" },
      ],
    },
    {
      title: "Integrations",
      children: [
        { title: "MCP Server", slug: "mcp" },
      ],
    },
  ],

  defaultPage: "introduction",

  footer: {
    left: "AGPL-3.0 License",
    right: "den — Secure sandbox runtime for AI agents",
  },
};
