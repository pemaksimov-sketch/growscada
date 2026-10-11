# Automated API integration (task 30)

`make test_api` builds the actual server and WASM in `Dockerfile`, starts an
isolated PostgreSQL database, applies every migration, waits for server
readiness and runs the ordered Bruno collection in `bruno_collections/automated`.
Docker with the Compose v2 plugin and Bash/Make are required. No host database,
seed IDs or host ports are used. The script removes only its own Compose
project on success or failure; JUnit and container logs remain in `artifacts/api`.
Credentials in the test Compose file are disposable fixture credentials.

Open `bruno_collections/automated` in Bruno for a manual run against a **fresh,
disposable database**, or run Bruno CLI 4.2.1 there with
`bru run --bail --env-var baseURL=http://localhost:9090`.
Requests depend on IDs/versions captured from previous responses; do not run
them in parallel. The exploratory `growscada_server` collection remains for
manual requests, including the intentionally open-ended SSE stream.

The ordered suite also exercises the project catalog, identical tag names in
two projects, foreign-resource reads/writes, project-scoped bindings, shared
Library validation, optimistic metadata conflicts and isolated cascading delete.
Legacy routes continue to address the seeded Main project.

GitHub and GitVerse workflow files invoke the same script. GitVerse execution
requires a copy of the repository and a Linux runner with Docker/Compose;
adding its workflow alone does not provision an account or a runner.

References: [Bruno CLI options](https://docs.usebruno.com/bru-cli/run/options),
[GitVerse workflows](https://gitverse.ru/docs/cicd/quick-start/).
