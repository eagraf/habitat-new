import { createFileRoute, Link } from "@tanstack/react-router";

export const Route = createFileRoute("/")({
  async beforeLoad({ search, context }) {
    if ("code" in search) {
      await context.authManager.exchangeCode(window.location.href);
      window.location.search = "";
    }
  },
  async loader() {
    return [
      {
        id: "permissions",
        name: "Permissions",
        description: "Manage permissions for privi",
        icon: "🔑",
        link: "/permissions",
      },
      {
        id: "privi-test",
        name: "Privi Test",
        description: "Privi Test for getting and putting records",
        icon: "💿",
        link: "/privi-test",
      },
      {
        id: "blob-test",
        name: "Blob Test",
        description: "Test uploading / getting blobs",
        icon: "📸",
        link: "/blob-test",
      },
    ];
  },
  component() {
    const data = Route.useLoaderData();
    return (
      <>
        <h1>Shortcuts</h1>
        <table>
          <thead>
            <tr>
              <th>App</th>
              <th>Description</th>
            </tr>
          </thead>
          <tbody>
            {data.map(({ id, name, description, icon, link }) => (
              <tr key={id}>
                <td>
                  <Link to={link}>
                    {icon} {name}
                  </Link>
                </td>
                <td>{description}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </>
    );
  },
});
