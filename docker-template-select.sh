#!/bin/sh

# Copyright 2026 Abera/Corteza contributors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0

is_abera_identifier() {
	case "$1" in
		"" | *[!a-z0-9-]* | -* | *-) return 1 ;;
		*) return 0 ;;
	esac
}

append_abera_provision_path() {
	path_to_add=$1
	case ":${PROVISION_PATH}:" in
		*":${path_to_add}:"*) ;;
		*) PROVISION_PATH="${PROVISION_PATH}:${path_to_add}" ;;
	esac
}

configure_abera_template_mode() {
	template_root=$1
	data_root=$2
	deployment_marker=$3
	requested_template=${ABERA_TEMPLATE:-}
	installed_descriptor=$4

	if [ -z "$requested_template" ] && [ "${installed_descriptor#template:}" != "$installed_descriptor" ]; then
		requested_template=${installed_descriptor#template:}
	fi

	if [ -z "$requested_template" ]; then
		echo "ABERA_TEMPLATE es obligatoria cuando ABERA_MODE=template" >&2
		return 64
	fi

	if ! is_abera_identifier "$requested_template"; then
		echo "ABERA_TEMPLATE inválida: use únicamente letras minúsculas, números y guiones" >&2
		return 64
	fi

	template_dir="${template_root}/${requested_template}"
	provision_dir="${template_dir}/provision"
	if [ ! -f "${template_dir}/manifest.yaml" ] || [ ! -d "$provision_dir" ]; then
		echo "No existe la plantilla '${requested_template}' en ${template_root}" >&2
		return 66
	fi

	requested_descriptor="template:${requested_template}"
	if [ -n "$installed_descriptor" ] && [ "$installed_descriptor" != "$requested_descriptor" ]; then
		echo "El volumen ya usa '${installed_descriptor}'; no se puede cambiar a '${requested_descriptor}'" >&2
		return 65
	fi

	append_abera_provision_path "$provision_dir"
	printf '%s\n' "$requested_descriptor" > "$deployment_marker"
	printf '%s\n' "$requested_template" > "${data_root}/.abera-template"

	ABERA_MODE=template
	ABERA_TEMPLATE=$requested_template
	export ABERA_MODE ABERA_TEMPLATE PROVISION_PATH
	echo "Plantilla seleccionada: ${requested_template}" >&2
}

configure_abera_demo_mode() {
	template_root=$1
	demo_root=$2
	deployment_marker=$3
	installed_descriptor=$4
	bundle=${ABERA_DEMO_BUNDLE:-}
	if [ -z "$bundle" ] && [ "${installed_descriptor#demo:}" != "$installed_descriptor" ]; then
		bundle=${installed_descriptor#demo:}
	fi
	[ -n "$bundle" ] || bundle=showroom-co

	if [ -n "${ABERA_TEMPLATE:-}" ]; then
		echo "ABERA_TEMPLATE no debe definirse cuando ABERA_MODE=demo" >&2
		return 64
	fi
	if ! is_abera_identifier "$bundle"; then
		echo "ABERA_DEMO_BUNDLE inválida: use únicamente letras minúsculas, números y guiones" >&2
		return 64
	fi

	bundle_dir="${demo_root}/${bundle}"
	template_list="${bundle_dir}/templates.list"
	if [ ! -f "${bundle_dir}/manifest.yaml" ] || [ ! -f "$template_list" ]; then
		echo "No existe el bundle demo '${bundle}' en ${demo_root}" >&2
		return 66
	fi

	requested_descriptor="demo:${bundle}"
	if [ -n "$installed_descriptor" ] && [ "$installed_descriptor" != "$requested_descriptor" ]; then
		echo "El volumen ya usa '${installed_descriptor}'; no se puede cambiar a '${requested_descriptor}'" >&2
		return 65
	fi

	template_count=0
	while IFS= read -r template_id || [ -n "$template_id" ]; do
		template_id=$(printf '%s' "$template_id" | sed 's/[[:space:]]*#.*$//; s/^[[:space:]]*//; s/[[:space:]]*$//')
		[ -n "$template_id" ] || continue

		if ! is_abera_identifier "$template_id"; then
			echo "Identificador inválido '${template_id}' en ${template_list}" >&2
			return 64
		fi

		template_dir="${template_root}/${template_id}"
		if [ ! -f "${template_dir}/manifest.yaml" ] || [ ! -d "${template_dir}/provision" ]; then
			echo "La demo referencia una plantilla inexistente: '${template_id}'" >&2
			return 66
		fi

		append_abera_provision_path "${template_dir}/provision"
		if [ -d "${template_dir}/demo/provision" ]; then
			append_abera_provision_path "${template_dir}/demo/provision"
		fi
		template_count=$((template_count + 1))
	done < "$template_list"

	if [ "$template_count" -eq 0 ]; then
		echo "El bundle demo '${bundle}' no contiene plantillas" >&2
		return 66
	fi
	if [ -d "${bundle_dir}/provision" ]; then
		append_abera_provision_path "${bundle_dir}/provision"
	fi

	printf '%s\n' "$requested_descriptor" > "$deployment_marker"
	ABERA_MODE=demo
	ABERA_DEMO_BUNDLE=$bundle
	export ABERA_MODE ABERA_DEMO_BUNDLE PROVISION_PATH
	echo "Demo seleccionada: ${bundle} (${template_count} plantillas)" >&2
}

configure_abera_deployment() {
	template_root=${ABERA_TEMPLATE_ROOT:-/corteza/templates}
	demo_root=${ABERA_DEMO_ROOT:-/corteza/demos}
	data_root=${STORAGE_PATH:-/data}
	deployment_marker="${data_root}/.abera-deployment"
	legacy_template_marker="${data_root}/.abera-template"
	requested_mode=${ABERA_MODE:-}
	installed_descriptor=

	mkdir -p "$data_root"
	if [ -f "$deployment_marker" ]; then
		installed_descriptor=$(sed -n '1p' "$deployment_marker")
	elif [ -f "$legacy_template_marker" ]; then
		installed_template=$(sed -n '1p' "$legacy_template_marker")
		[ -z "$installed_template" ] || installed_descriptor="template:${installed_template}"
	fi

	if [ -z "$requested_mode" ]; then
		case "$installed_descriptor" in
			template:*) requested_mode=template ;;
			demo:*) requested_mode=demo ;;
			*) [ -z "${ABERA_TEMPLATE:-}" ] || requested_mode=template ;;
		esac
	fi
	if [ -z "$requested_mode" ]; then
		return 0
	fi

	PROVISION_PATH=${PROVISION_PATH:-/corteza/provision/*}
	case "$requested_mode" in
		template)
			configure_abera_template_mode "$template_root" "$data_root" "$deployment_marker" "$installed_descriptor"
			;;
		demo)
			configure_abera_demo_mode "$template_root" "$demo_root" "$deployment_marker" "$installed_descriptor"
			;;
		*)
			echo "ABERA_MODE inválido: use 'template' o 'demo'" >&2
			return 64
			;;
	esac
}

# Backwards-compatible entrypoint for existing tests and deployments.
configure_abera_template() {
	configure_abera_deployment
}
