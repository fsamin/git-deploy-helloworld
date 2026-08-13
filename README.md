# git-deploy-helloworld

Demo application for
[git-deploy-operator](https://github.com/fsamin/git-deploy-operator), a Kubernetes
operator PoC that builds and deploys applications straight from a git repository.

It answers on port `8080` (declared via `EXPOSE` in the Dockerfile, which the
operator uses to discover the port to expose), and its one page showcases
everything the operator can inject:

- **Environment variables** — every variable is listed, with sensitive-looking
  values and URL passwords masked. Try `git-deploy env set GREETING=hi` and
  `git-deploy env set API_KEY=s3cr3t --secret`.
- **Configuration files** — the page prints `/etc/app/config.yaml` when it is
  mounted, and a `greeting:` line in it changes the headline. Try
  `git-deploy file set /etc/app/config.yaml --from ./config.yaml`.
- **PostgreSQL add-on** — when `DATABASE_URL` is injected
  (`git-deploy addon add postgresql`), the page counts visits in the database.
- **Redis add-on** — when `REDIS_URL` is injected
  (`git-deploy addon add redis`), the page counts hits in redis (a minimal
  RESP exchange, no client library).
- **Logs** — startup reports what is injected, every request is logged:
  `git-deploy logs -f` has something to say.

The `no-expose` branch carries a Dockerfile without `EXPOSE`, used to demo the
operator's explicit failure path.
