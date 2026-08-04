#!/bin/sh

# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
. "${repo_root}/docker-template-select.sh"

fail() {
	echo "FAILED: $1" >&2
	exit 1
}

assert_eq() {
	[ "$1" = "$2" ] || fail "expected '$2', got '$1'"
}

test_root=$(mktemp -d)
trap 'rm -rf "$test_root"' EXIT

template_root="${test_root}/templates"
demo_root="${test_root}/demos"
data_root="${test_root}/data"
template_ids="
inmobiliaria-co
automotriz-co
admisiones-educativas-co
servicios-tecnicos-co
centro-contacto-co
soporte-renovaciones-co
"
for template_id in $template_ids; do
	mkdir -p "${template_root}/${template_id}/provision"
	mkdir -p "${template_root}/${template_id}/demo/provision"
	: > "${template_root}/${template_id}/manifest.yaml"
done
mkdir -p "${demo_root}/showroom-co/provision"
: > "${demo_root}/showroom-co/manifest.yaml"
printf '%s\n' $template_ids > "${demo_root}/showroom-co/templates.list"
mkdir -p "${demo_root}/cliente-co/provision"
: > "${demo_root}/cliente-co/manifest.yaml"
printf '%s\n' inmobiliaria-co > "${demo_root}/cliente-co/templates.list"
mkdir -p "$data_root"

ABERA_TEMPLATE_ROOT=$template_root
ABERA_DEMO_ROOT=$demo_root
STORAGE_PATH=$data_root
ABERA_TEMPLATE=inmobiliaria-co
PROVISION_PATH="/corteza/provision/*"
export ABERA_TEMPLATE_ROOT ABERA_DEMO_ROOT STORAGE_PATH ABERA_TEMPLATE PROVISION_PATH

configure_abera_template
assert_eq "$ABERA_TEMPLATE" "inmobiliaria-co"
assert_eq "$PROVISION_PATH" "/corteza/provision/*:${template_root}/inmobiliaria-co/provision"
assert_eq "$(sed -n '1p' "${data_root}/.abera-template")" "inmobiliaria-co"
assert_eq "$(sed -n '1p' "${data_root}/.abera-deployment")" "template:inmobiliaria-co"

# Repeated configuration must not duplicate the provision path.
configure_abera_template
assert_eq "$PROVISION_PATH" "/corteza/provision/*:${template_root}/inmobiliaria-co/provision"

# A restart may recover the selected template from the persistent marker.
ABERA_TEMPLATE=
PROVISION_PATH="/corteza/provision/*"
configure_abera_template
assert_eq "$ABERA_TEMPLATE" "inmobiliaria-co"

# Every public template identifier must be selectable on its own clean volume.
for template_id in $template_ids; do
	STORAGE_PATH="${test_root}/data-${template_id}"
	ABERA_TEMPLATE=$template_id
	PROVISION_PATH="/corteza/provision/*"
	export STORAGE_PATH ABERA_TEMPLATE PROVISION_PATH
	configure_abera_template
	assert_eq "$ABERA_TEMPLATE" "$template_id"
	assert_eq "$PROVISION_PATH" "/corteza/provision/*:${template_root}/${template_id}/provision"
	assert_eq "$(sed -n '1p' "${STORAGE_PATH}/.abera-template")" "$template_id"
done

# Demo mode installs the declared bundle and every template's demo overlay.
STORAGE_PATH="${test_root}/data-demo"
ABERA_MODE=demo
ABERA_TEMPLATE=
ABERA_DEMO_BUNDLE=showroom-co
PROVISION_PATH="/corteza/provision/*"
export STORAGE_PATH ABERA_MODE ABERA_TEMPLATE ABERA_DEMO_BUNDLE PROVISION_PATH
configure_abera_deployment
assert_eq "$(sed -n '1p' "${STORAGE_PATH}/.abera-deployment")" "demo:showroom-co"
for template_id in $template_ids; do
	case ":${PROVISION_PATH}:" in
		*":${template_root}/${template_id}/provision:"*) ;;
		*) fail "demo missing template provision path for ${template_id}" ;;
	esac
	case ":${PROVISION_PATH}:" in
		*":${template_root}/${template_id}/demo/provision:"*) ;;
		*) fail "demo missing data provision path for ${template_id}" ;;
	esac
done
case ":${PROVISION_PATH}:" in
	*":${demo_root}/showroom-co/provision:"*) ;;
	*) fail "demo missing bundle provision path" ;;
esac

# Demo mode is restored from its marker and cannot be mixed with a template.
ABERA_MODE=
PROVISION_PATH="/corteza/provision/*"
configure_abera_deployment
assert_eq "$ABERA_MODE" "demo"

ABERA_MODE=template
ABERA_TEMPLATE=inmobiliaria-co
if configure_abera_deployment 2>/dev/null; then
	fail "demo volume accepted a template deployment"
fi

# A custom demo bundle is restored from its marker when its environment
# variable is omitted on a subsequent restart.
STORAGE_PATH="${test_root}/data-demo-custom"
ABERA_MODE=demo
ABERA_TEMPLATE=
ABERA_DEMO_BUNDLE=cliente-co
PROVISION_PATH="/corteza/provision/*"
export STORAGE_PATH ABERA_MODE ABERA_TEMPLATE ABERA_DEMO_BUNDLE PROVISION_PATH
configure_abera_deployment
unset ABERA_MODE ABERA_DEMO_BUNDLE
PROVISION_PATH="/corteza/provision/*"
configure_abera_deployment
assert_eq "$ABERA_MODE" "demo"
assert_eq "$ABERA_DEMO_BUNDLE" "cliente-co"

# Invalid identifiers, missing templates and volume mismatches must fail.
STORAGE_PATH=$data_root
ABERA_MODE=template
ABERA_TEMPLATE="../inmobiliaria-co"
export STORAGE_PATH ABERA_TEMPLATE
if configure_abera_template 2>/dev/null; then
	fail "path traversal template was accepted"
fi

ABERA_TEMPLATE=no-existe
if configure_abera_template 2>/dev/null; then
	fail "missing template was accepted"
fi

ABERA_TEMPLATE=centro-contacto-co
if configure_abera_template 2>/dev/null; then
	fail "template switch on an initialized volume was accepted"
fi

echo "docker-template-select: OK"
