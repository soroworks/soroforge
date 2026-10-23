# API timeout invariants

The HTTP server keeps `ReadHeaderTimeout` enabled to limit Slowloris exposure.
`WriteTimeout` must remain longer than the deploy request timeout so a slow but
valid deployment can return its result. Tests should pin both relationships so
future configuration changes cannot silently weaken them.
