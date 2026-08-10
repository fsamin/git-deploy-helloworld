# git-deploy-helloworld

Minimal HTTP server used as the demo application for
[git-deploy-operator](https://github.com/fsamin/git-deploy-operator), a Kubernetes
operator PoC that builds and deploys applications straight from a git repository.

It answers on port `8080` (declared via `EXPOSE` in the Dockerfile, which the
operator uses to discover the port to expose).

The `no-expose` branch carries a Dockerfile without `EXPOSE`, used to demo the
operator's explicit failure path.
