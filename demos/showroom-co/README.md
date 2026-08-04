# Showroom comercial Abera

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See the repository `LICENSE`.

This bundle is selected with `ABERA_MODE=demo` and
`ABERA_DEMO_BUNDLE=showroom-co`. On a new volume it provisions every template
listed in `templates.list`, demo users and deterministic synthetic records.

The bundle is exclusively for demonstration and testing. Customer deployments
must use `ABERA_MODE=template` with a single `ABERA_TEMPLATE` and do not receive
the synthetic datasets.
