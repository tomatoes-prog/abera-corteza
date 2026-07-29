#!/bin/sh

# Copyright 2026 Abera/Corteza contributors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0

set -eu

WEBAPP_DIR=${HTTP_WEBAPP_BASE_DIR:-/corteza/webapp}
DEFAULT_LOCALE=${CORTEZA_DEFAULT_LOCALE:-es}

# The built webapps intentionally do not contain deployment-specific config.js
# files. Generate them at runtime and expose the same default language to all
# applications before their JavaScript bundles are evaluated.
for app in admin reporter compose workflow discovery privacy .; do
	app_dir="${WEBAPP_DIR}/${app}"
	config="${app_dir}/config.js"

	if [ ! -f "$config" ]; then
		{
			printf "window.CortezaAPI = '/api';\n"
			printf "window.CortezaDiscoveryAPI = '/api';\n"
		} > "$config"
	fi

	# Keep restarts idempotent and allow an explicitly overridden environment
	# value to take effect on the next container start.
	sed -i '/^[[:space:]]*window\.CortezaLocale[[:space:]]*=/d' "$config"
	printf "window.CortezaLocale = '%s';\n" "$DEFAULT_LOCALE" >> "$config"

	# Corteza serves config.js dynamically and may replace the generated file.
	# Put the locale in the page itself so it is available before the bundles
	# initialize, regardless of how config.js is served.
	index="${app_dir}/index.html"
	if [ -f "$index" ]; then
		sed -i "s|<script>window\.CortezaLocale = '[^']*';</script>||g" "$index"
		sed -i "s|</head>|<script>window.CortezaLocale = '${DEFAULT_LOCALE}';</script></head>|" "$index"
	fi
done

exec /corteza/bin/corteza-server "$@"
