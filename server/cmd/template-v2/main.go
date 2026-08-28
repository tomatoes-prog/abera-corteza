// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command template-v2 is a development-only materializer for the six explicit
// Abera template definitions. It updates version-controlled YAML and is never
// invoked by Docker, the entrypoint, provisioning, or a customer deployment.
// Runtime selection installs the committed resources verbatim; it does not
// generate a common design or mutate an existing customer database.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const licenseHeader = `# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

`

const (
	homeMetricHeight          = 18
	homeOperationalListHeight = 48
)

var templateIDs = []string{
	"inmobiliaria-co",
	"automotriz-co",
	"admisiones-educativas-co",
	"servicios-tecnicos-co",
	"centro-contacto-co",
	"soporte-renovaciones-co",
}

var modulePurpose = map[string]string{
	"inmobiliaria-co/inmuebles":                     "Inventario comercial de inmuebles disponibles para venta o arriendo. Se crea un registro por inmueble y se relaciona con los leads interesados, sus visitas, actividades y negociaciones.",
	"inmobiliaria-co/leads":                         "Personas interesadas en comprar o arrendar inmuebles. Centraliza sus datos de contacto, autorización, necesidad, presupuesto, inmuebles de interés, responsable y avance comercial.",
	"inmobiliaria-co/citas":                         "Visitas, llamadas o reuniones acordadas con un lead. Una cita puede incluir varios inmuebles y conserva responsable, horario, estado, resultado y observaciones.",
	"inmobiliaria-co/actividades":                   "Historial y agenda de seguimientos comerciales realizados con un lead, opcionalmente relacionados con un inmueble. Permite controlar responsables, vencimientos y resultados.",
	"inmobiliaria-co/negociaciones":                 "Oportunidades comerciales abiertas para un lead y un inmueble concretos. Registra valores, etapa, probabilidad, próximo paso y resultado de venta o arriendo.",
	"automotriz-co/vehiculos":                       "Inventario de vehículos nuevos o usados. Reúne identificación, características, precio, sede, disponibilidad, cliente final y trazabilidad de pruebas, oportunidades y posventa.",
	"automotriz-co/prospectos":                      "Personas interesadas en adquirir un vehículo. Centraliza consentimiento, presupuesto, financiación, vehículos de interés, responsable y estado del proceso comercial.",
	"automotriz-co/pruebas-manejo":                  "Reservas de vehículos para pruebas de manejo. Relaciona prospecto, vehículo y asesor, evita cruces de horario y registra el resultado y la siguiente acción.",
	"automotriz-co/oportunidades":                   "Negociaciones de venta de un vehículo a un prospecto. Contiene oferta, financiación, retoma, probabilidad, etapa y fecha estimada de cierre.",
	"automotriz-co/ordenes-servicio":                "Citas y trabajos de taller asociados con un cliente y su vehículo. Registra programación, diagnóstico, ejecución, evidencias, valores y próximo mantenimiento.",
	"automotriz-co/actividades-comerciales":         "Seguimientos comerciales relacionados con un prospecto y, opcionalmente, un vehículo u oportunidad. Conserva responsable, fecha, resultado y próxima acción.",
	"automotriz-co/tareas-servicio":                 "Checklist operativo de una orden de servicio. Cada tarea identifica responsable, obligatoriedad, estado, resultado y evidencia antes del cierre de la orden.",
	"admisiones-educativas-co/prospectos":           "Personas interesadas en la oferta educativa. Centraliza estudiante, acudiente cuando aplica, autorización, programa principal, programas de interés, asesor y avance hacia la matrícula.",
	"admisiones-educativas-co/programas":            "Oferta académica disponible para captación y matrícula. Registra nivel, modalidad, sede, periodo, valor, cupos y meta comercial.",
	"admisiones-educativas-co/solicitudes":          "Postulaciones formales de un prospecto a un programa y periodo. Consolida requisitos, completitud, revisión, decisión y observaciones del proceso de admisión.",
	"admisiones-educativas-co/citas-admision":       "Llamadas, entrevistas, visitas o sesiones virtuales del proceso de admisión. Relaciona prospecto, programa, asesor, horario, resultado y siguiente acción.",
	"admisiones-educativas-co/matriculas":           "Confirmaciones académicas y financieras derivadas de una solicitud admitida. Registra programa, periodo, valor, beneficios, pago y estado de matrícula.",
	"admisiones-educativas-co/requisitos-solicitud": "Documentos o condiciones individuales que debe cumplir una solicitud. Permite revisar cada requisito, adjuntar evidencia y registrar aceptación o rechazo.",
	"admisiones-educativas-co/actividades-admision": "Historial de seguimientos realizados con un prospecto durante captación y admisión, relacionado opcionalmente con un programa o solicitud.",
	"servicios-tecnicos-co/clientes-prospectos":     "Personas o empresas que solicitan, cotizan o reciben servicios. Es el punto de entrada para activos, solicitudes, cotizaciones y órdenes de trabajo.",
	"servicios-tecnicos-co/activos":                 "Equipos instalados o atendidos para un cliente. Mantiene identificación, criticidad, ubicación, garantía e historial de solicitudes, órdenes y mantenimiento.",
	"servicios-tecnicos-co/solicitudes":             "Necesidades de instalación, diagnóstico, reparación o mantenimiento reportadas por un cliente. Controla prioridad, responsable, ubicación, estado y SLA.",
	"servicios-tecnicos-co/cotizaciones":            "Propuestas económicas asociadas con una solicitud. Resume alcance, vigencia, impuestos, total, aceptación y líneas detalladas de productos o servicios.",
	"servicios-tecnicos-co/ordenes-trabajo":         "Trabajos programados para ejecutar una solicitud aceptada. Relaciona cliente, activo, técnico, agenda, ubicación, diagnóstico, tareas, evidencias y cierre.",
	"servicios-tecnicos-co/items-cotizacion":        "Líneas económicas de una cotización. Se usa una línea por mano de obra, material, desplazamiento u otro concepto, con cantidad, valor e impuestos.",
	"servicios-tecnicos-co/tareas-orden":            "Checklist de ejecución de una orden de trabajo. Registra responsable, obligatoriedad, tiempos, resultado y evidencias de cada actividad técnica.",
	"centro-contacto-co/contactos-leads":            "Personas cargadas para gestión telefónica o comercial. Centraliza autorización, canales permitidos, validación RNE, intentos, responsable y estado de conversión.",
	"centro-contacto-co/campanas":                   "Iniciativas de contacto con objetivo, producto, guion, vigencia, horario, prioridad y metas. Agrupa registros, llamadas y oportunidades.",
	"centro-contacto-co/registros-campana":          "Relación operativa entre una campaña y un contacto. Funciona como cola de trabajo y conserva agente, prioridad, intentos, próxima gestión y resultado acumulado.",
	"centro-contacto-co/llamadas":                   "Historial de intentos y conversaciones de una campaña. Registra agente, horario, duración, disposición, resumen, callback e identificadores externos opcionales.",
	"centro-contacto-co/oportunidades":              "Oportunidades comerciales originadas en llamadas calificadas. Relaciona contacto, campaña, producto, responsable, valor, etapa y cierre esperado.",
	"soporte-renovaciones-co/clientes":              "Personas o empresas atendidas por soporte. Consolida responsable, plan, valor recurrente, salud, satisfacción, riesgo, contratos, casos y renovaciones.",
	"soporte-renovaciones-co/contratos":             "Acuerdos de servicio vigentes para un cliente y producto. Define cobertura, fechas, valor y objetivos de respuesta y resolución que gobiernan los casos.",
	"soporte-renovaciones-co/casos":                 "Solicitudes o incidentes reportados por clientes. Controla clasificación, responsable, prioridad, SLA, escalamiento, interacciones, causa y resolución.",
	"soporte-renovaciones-co/interacciones":         "Historial cronológico de comunicaciones y acciones sobre un caso o renovación. Permite distinguir respuestas del agente y conservar archivos.",
	"soporte-renovaciones-co/renovaciones":          "Proceso comercial previo al vencimiento de un contrato. Registra responsable, etapa, valor, probabilidad, riesgo, expansión, próxima gestión y resultado.",
	"soporte-renovaciones-co/problemas-conocidos":   "Problemas recurrentes que agrupan varios casos bajo una causa, impacto, solución temporal y solución definitiva comunes.",
}

var automaticFields = map[string]bool{
	"fechaAsignacion": true, "vencimientoSLA": true, "slaIncumplido": true,
	"primeraRespuesta": true, "fechaResolucion": true,
	"vencimientoRespuesta": true, "vencimientoResolucion": true,
	"umbralRespuestaSLA": true, "umbralResolucionSLA": true,
	"estadoSLA": true, "duracionSegundos": true, "totalIntentos": true,
	"estadoIntegracion": true, "ultimoErrorIntegracion": true,
}

type metricSpec struct {
	label, description, module, filter, metricField, operation, prefix, suffix string
}

type organizerSpec struct {
	page, title, module, labelField, descriptionField, groupField, filter string
}

type mapSpec struct {
	page, title, module, titleField, geometryField, filter string
	center                                                 []float64
	bounds                                                 [][]float64
	zoomStarting                                           int
}

type relationSpec struct {
	page, title, module, prefilter, presort, refField string
	fields                                            []string
}

type recordFieldsSpec struct {
	page   string
	fields []string
}

type auxiliaryPageSpec struct {
	parent, handle, title, module, description string
	fields                                     []string
	visible                                    bool
}

type queuePageSpec struct {
	handle, title, description, module, prefilter, presort string
	fields                                                 []string
}

type visualConfig struct {
	metrics      []metricSpec
	reportCharts []string
	reportModule string
	reportFields []string
	organizer    organizerSpec
	mapBlock     mapSpec
	relations    []relationSpec
	recordFields []recordFieldsSpec
	auxiliary    []auxiliaryPageSpec
	queue        queuePageSpec
}

type workflowNoticeConfig struct {
	namespace, module, userField, runAs, recordLabel, milestone string
	milestoneModule, milestoneField, milestoneValue             string
}

var workflowNoticeConfigs = map[string]workflowNoticeConfig{
	"inmobiliaria-co":          {namespace: "inmobiliaria-co", module: "leads", userField: "responsable", runAs: "automatizacion-inmobiliaria", recordLabel: "lead", milestone: "negociación ganada", milestoneModule: "negociaciones", milestoneField: "estado", milestoneValue: "Ganada"},
	"automotriz-co":            {namespace: "automotriz-co", module: "prospectos", userField: "asesor", runAs: "automatizacion-automotriz", recordLabel: "prospecto", milestone: "venta confirmada", milestoneModule: "oportunidades", milestoneField: "etapa", milestoneValue: "Vendido"},
	"admisiones-educativas-co": {namespace: "admisiones-educativas-co", module: "prospectos", userField: "asesor", runAs: "automatizacion-admisiones", recordLabel: "prospecto", milestone: "matrícula confirmada", milestoneModule: "matriculas", milestoneField: "estado", milestoneValue: "Confirmada"},
	"servicios-tecnicos-co":    {namespace: "servicios-tecnicos-co", module: "solicitudes", userField: "responsable", runAs: "automatizacion-servicios", recordLabel: "solicitud", milestone: "orden completada", milestoneModule: "ordenes-trabajo", milestoneField: "estado", milestoneValue: "Completada"},
	"centro-contacto-co":       {namespace: "centro-contacto-co", module: "registros-campana", userField: "agente", runAs: "automatizacion-contacto", recordLabel: "gestión de campaña", milestone: "venta lograda", milestoneModule: "oportunidades", milestoneField: "etapa", milestoneValue: "Venta"},
	"soporte-renovaciones-co":  {namespace: "soporte-renovaciones-co", module: "casos", userField: "agente", runAs: "automatizacion-soporte", recordLabel: "caso", milestone: "renovación confirmada", milestoneModule: "renovaciones", milestoneField: "etapa", milestoneValue: "Renovada"},
}

var visualConfigs = map[string]visualConfig{
	"inmobiliaria-co": {
		metrics: []metricSpec{
			{label: "Leads activos", description: "Leads con gestión comercial abierta.", module: "leads", filter: "estado != 'Ganado' AND estado != 'Perdido' AND estado != 'Descartado'", metricField: "count", operation: "count"},
			{label: "Leads sin asignar", description: "Leads que todavía no tienen un asesor responsable.", module: "leads", filter: "responsable IS NULL", metricField: "count", operation: "count"},
			{label: "Visitas próximas", description: "Citas pendientes o confirmadas para visitar inmuebles.", module: "citas", filter: "estado = 'Pendiente' OR estado = 'Confirmada'", metricField: "count", operation: "count"},
			{label: "Pipeline", description: "Valor de las negociaciones abiertas en COP.", module: "negociaciones", filter: "estado = 'Abierta'", metricField: "valorOfrecido", operation: "sum", prefix: "$ "},
		},
		reportCharts: []string{"leads-por-estado", "inmuebles-por-estado", "negociaciones-por-etapa"},
		reportModule: "negociaciones", reportFields: []string{"lead", "inmueble", "asesor", "etapa", "valorOfrecido", "probabilidad", "fechaEstimadaCierre"},
		organizer: organizerSpec{page: "negociaciones", title: "Pipeline comercial", module: "negociaciones", labelField: "lead", descriptionField: "proximoPaso", groupField: "etapa", filter: "estado = 'Abierta'"},
		mapBlock: mapSpec{
			page: "inmuebles", title: "Mapa de inventario disponible", module: "inmuebles",
			titleField: "nombre", geometryField: "ubicacion", filter: "estado = 'Disponible'",
			center:       []float64{4.5709, -74.2973},
			bounds:       [][]float64{{13.7, -66.8}, {-4.3, -79.1}},
			zoomStarting: 5,
		},
		recordFields: []recordFieldsSpec{
			{page: "leads/lead-registro", fields: []string{"fechaAutorizacion", "origenAutorizacion", "prioridad", "ultimoContacto", "motivoCierre"}},
			{page: "inmuebles/inmueble-registro", fields: []string{"codigoDivipola", "sector", "ubicacion", "disponibleDesde", "destacado", "caracteristicas"}},
			{page: "actividades/actividad-registro", fields: []string{"asunto", "estado", "prioridad", "vencimiento"}},
			{page: "negociaciones/negociacion-registro", fields: []string{"proximoPaso", "fechaCierreReal", "comisionEstimada"}},
		},
		relations: []relationSpec{
			{page: "inmuebles/inmueble-registro", title: "Leads interesados", module: "leads", prefilter: "inmueblesInteres = ${recordID}", presort: "createdAt DESC", fields: []string{"nombres", "apellidos", "telefono", "estado", "responsable", "presupuestoMaximo"}},
			{page: "inmuebles/inmueble-registro", title: "Citas del inmueble", module: "citas", prefilter: "inmuebles = ${recordID}", presort: "inicio DESC", fields: []string{"asunto", "lead", "inicio", "estado", "asesor"}},
			{page: "inmuebles/inmueble-registro", title: "Actividades del inmueble", module: "actividades", prefilter: "inmueble = ${recordID}", presort: "fecha DESC", fields: []string{"asunto", "lead", "tipo", "estado", "fecha", "asesor"}},
			{page: "inmuebles/inmueble-registro", title: "Negociaciones del inmueble", module: "negociaciones", prefilter: "inmueble = ${recordID}", presort: "createdAt DESC", fields: []string{"lead", "asesor", "etapa", "valorOfrecido", "probabilidad", "estado"}},
		},
	},
	"automotriz-co": {
		metrics: []metricSpec{
			{label: "Prospectos sin asignar", description: "Prospectos que todavía no tienen un asesor responsable.", module: "prospectos", filter: "asesor IS NULL", metricField: "count", operation: "count"},
			{label: "Pruebas próximas", description: "Pruebas de manejo pendientes o confirmadas.", module: "pruebas-manejo", filter: "estado = 'Pendiente' OR estado = 'Confirmada'", metricField: "count", operation: "count"},
			{label: "Pipeline", description: "Valor de las oportunidades comerciales abiertas en COP.", module: "oportunidades", filter: "etapa != 'Vendido' AND etapa != 'Perdido'", metricField: "valorOfertado", operation: "sum", prefix: "$ "},
			{label: "Vehículos disponibles", description: "Vehículos del inventario disponibles para venta.", module: "vehiculos", filter: "estado = 'Disponible'", metricField: "count", operation: "count"},
		},
		reportCharts: []string{"funnel-ventas", "oportunidades-por-etapa", "inventario-por-estado", "servicio-por-estado"},
		reportModule: "ordenes-servicio", reportFields: []string{"codigo", "cliente", "vehiculo", "tecnico", "inicio", "estado", "valor", "proximoMantenimiento"},
		organizer: organizerSpec{page: "oportunidades", title: "Pipeline de oportunidades", module: "oportunidades", labelField: "prospecto", descriptionField: "proximoPaso", groupField: "etapa", filter: "etapa != 'Vendido' AND etapa != 'Perdido'"},
		recordFields: []recordFieldsSpec{
			{page: "prospectos/prospecto-registro", fields: []string{"fechaAutorizacion", "origenAutorizacion", "prioridad", "ultimoContacto", "motivoCierre"}},
			{page: "oportunidades/oportunidad-registro", fields: []string{"proximoPaso", "fechaCierreReal"}},
			{page: "ordenes-servicio/orden-registro", fields: []string{"kilometrajeIngreso", "inicioReal", "finReal", "evidencias", "firmaEntrega", "fechaEntrega"}},
		},
		relations: []relationSpec{
			{page: "prospectos/prospecto-registro", title: "Actividades comerciales", module: "actividades-comerciales", prefilter: "prospecto = ${recordID}", presort: "fecha DESC", refField: "prospecto", fields: []string{"asunto", "vehiculo", "oportunidad", "asesor", "tipo", "estado", "fecha", "proximaAccion"}},
			{page: "vehiculos/vehiculo-registro", title: "Actividades relacionadas", module: "actividades-comerciales", prefilter: "vehiculo = ${recordID}", presort: "fecha DESC", refField: "vehiculo", fields: []string{"asunto", "prospecto", "oportunidad", "asesor", "estado", "fecha"}},
			{page: "ordenes-servicio/orden-registro", title: "Checklist de servicio", module: "tareas-servicio", prefilter: "orden = ${recordID}", presort: "secuencia ASC", refField: "orden", fields: []string{"secuencia", "tarea", "obligatoria", "tecnico", "estado", "finReal"}},
		},
		auxiliary: []auxiliaryPageSpec{
			{parent: "prospectos", handle: "actividad-comercial-registro", title: "Detalle de actividad comercial", module: "actividades-comerciales", description: "Edición y consulta de un seguimiento comercial relacionado con prospecto, vehículo u oportunidad.", fields: []string{"asunto", "prospecto", "vehiculo", "oportunidad", "asesor", "tipo", "estado", "fecha", "resultado", "proximaAccion", "observaciones"}},
			{parent: "ordenes-servicio", handle: "tarea-servicio-registro", title: "Detalle de tarea de servicio", module: "tareas-servicio", description: "Edición y consulta de una tarea del checklist técnico de la orden.", fields: []string{"orden", "secuencia", "tarea", "obligatoria", "tecnico", "estado", "inicioReal", "finReal", "resultado", "evidencia"}},
		},
	},
	"admisiones-educativas-co": {
		metrics: []metricSpec{
			{label: "Prospectos activos", description: "Personas que continúan en el proceso de admisión.", module: "prospectos", filter: "estado != 'Matriculado' AND estado != 'No matriculado'", metricField: "count", operation: "count"},
			{label: "Solicitudes incompletas", description: "Solicitudes incompletas o pendientes de revisión.", module: "solicitudes", filter: "estado = 'Incompleta' OR estado = 'En revisión'", metricField: "count", operation: "count"},
			{label: "Admitidos pendientes", description: "Admitidos con matrícula o documentos pendientes.", module: "matriculas", filter: "estado = 'Preparada' OR estado = 'Documentos pendientes'", metricField: "count", operation: "count"},
			{label: "Matrículas confirmadas", description: "Estudiantes con matrícula confirmada.", module: "matriculas", filter: "estado = 'Confirmada'", metricField: "count", operation: "count"},
		},
		reportCharts: []string{"funnel-admisiones", "solicitudes-por-programa", "solicitudes-por-estado", "matriculas-por-programa"},
		reportModule: "solicitudes", reportFields: []string{"codigo", "prospecto", "programa", "periodo", "estado", "completitud", "revisor", "fechaDecision"},
		organizer: organizerSpec{page: "solicitudes", title: "Etapas de solicitudes", module: "solicitudes", labelField: "codigo", descriptionField: "periodo", groupField: "estado", filter: "estado != 'No admitida' AND estado != 'Retirada'"},
		recordFields: []recordFieldsSpec{
			{page: "programas/programa-registro", fields: []string{"jornada", "fechaInicio"}},
			{page: "prospectos/prospecto-registro", fields: []string{"tipoDocumento", "fechaNacimiento", "esMenorEdad", "fechaAutorizacion", "programaPrincipal", "prioridad", "ultimoContacto", "motivoCierre"}},
		},
		relations: []relationSpec{
			{page: "prospectos/prospecto-registro", title: "Actividades de admisión", module: "actividades-admision", prefilter: "prospecto = ${recordID}", presort: "fecha DESC", refField: "prospecto", fields: []string{"asunto", "programa", "solicitud", "asesor", "tipo", "estado", "fecha", "proximaAccion"}},
			{page: "solicitudes/solicitud-registro", title: "Checklist documental", module: "requisitos-solicitud", prefilter: "solicitud = ${recordID}", presort: "createdAt ASC", refField: "solicitud", fields: []string{"requisito", "obligatorio", "estado", "fechaRecepcion", "fechaValidacion", "revisor", "motivoRechazo"}},
			{page: "solicitudes/solicitud-registro", title: "Actividades de la solicitud", module: "actividades-admision", prefilter: "solicitud = ${recordID}", presort: "fecha DESC", refField: "solicitud", fields: []string{"asunto", "prospecto", "asesor", "tipo", "estado", "fecha", "resultado"}},
		},
		auxiliary: []auxiliaryPageSpec{
			{parent: "solicitudes", handle: "requisito-solicitud-registro", title: "Detalle de requisito", module: "requisitos-solicitud", description: "Recepción y validación de un requisito individual de la solicitud.", fields: []string{"solicitud", "requisito", "obligatorio", "estado", "archivo", "fechaRecepcion", "fechaValidacion", "revisor", "motivoRechazo", "observaciones"}},
			{parent: "prospectos", handle: "actividad-admision-registro", title: "Detalle de actividad de admisión", module: "actividades-admision", description: "Edición y consulta de una actividad de captación o admisión.", fields: []string{"asunto", "prospecto", "solicitud", "programa", "asesor", "tipo", "estado", "fecha", "resultado", "proximaAccion", "observaciones"}},
		},
	},
	"servicios-tecnicos-co": {
		metrics: []metricSpec{
			{label: "Solicitudes abiertas", description: "Solicitudes que todavía requieren gestión operativa.", module: "solicitudes", filter: "estado != 'Cerrada' AND estado != 'Perdida'", metricField: "count", operation: "count"},
			{label: "SLA en riesgo", description: "Solicitudes vencidas o próximas a incumplir su SLA.", module: "solicitudes", filter: "slaIncumplido = '1' OR vencimientoSLA < NOW()", metricField: "count", operation: "count"},
			{label: "Órdenes abiertas", description: "Órdenes de trabajo pendientes de completar.", module: "ordenes-trabajo", filter: "estado != 'Completada' AND estado != 'Cancelada'", metricField: "count", operation: "count"},
			{label: "Valor vendido", description: "Valor total de las cotizaciones aceptadas en COP.", module: "cotizaciones", filter: "estado = 'Aceptada'", metricField: "total", operation: "sum", prefix: "$ "},
		},
		reportCharts: []string{"funnel-servicios", "solicitudes-por-prioridad", "ordenes-por-estado", "valor-por-servicio"},
		reportModule: "ordenes-trabajo", reportFields: []string{"codigo", "cliente", "activo", "tecnico", "inicio", "fin", "estado", "valorFinal"},
		organizer: organizerSpec{page: "ordenes-trabajo", title: "Tablero de órdenes", module: "ordenes-trabajo", labelField: "codigo", descriptionField: "direccion", groupField: "estado", filter: "estado != 'Completada' AND estado != 'Cancelada'"},
		mapBlock:  mapSpec{page: "ordenes-trabajo", title: "Mapa de órdenes programadas", module: "ordenes-trabajo", titleField: "codigo", geometryField: "ubicacionTrabajo", filter: "estado != 'Completada' AND estado != 'Cancelada'", center: []float64{4.711, -74.0721}},
		recordFields: []recordFieldsSpec{
			{page: "clientes-prospectos/cliente-registro", fields: []string{"fechaAutorizacion", "codigoDivipola", "ubicacionServicio"}},
			{page: "activos/activo-registro", fields: []string{"criticidad", "ultimaIntervencion", "ubicacionMapa"}},
			{page: "solicitudes/solicitud-registro", fields: []string{"contactoEnSitio", "ubicacionServicio", "ventanaInicio", "ventanaFin", "requiereVisitaDiagnostico"}},
			{page: "ordenes-trabajo/orden-registro", fields: []string{"contactoEnSitio", "ubicacionTrabajo", "inicioReal", "finReal", "aceptacionCliente"}},
		},
		relations: []relationSpec{
			{page: "cotizaciones/cotizacion-registro", title: "Ítems de la cotización", module: "items-cotizacion", prefilter: "cotizacion = ${recordID}", presort: "secuencia ASC", refField: "cotizacion", fields: []string{"secuencia", "tipo", "descripcion", "cantidad", "valorUnitario", "impuestoPorcentaje", "total"}},
			{page: "ordenes-trabajo/orden-registro", title: "Checklist de ejecución", module: "tareas-orden", prefilter: "orden = ${recordID}", presort: "secuencia ASC", refField: "orden", fields: []string{"secuencia", "tarea", "obligatoria", "tecnico", "estado", "inicioReal", "finReal"}},
			{page: "activos/activo-registro", title: "Historial de órdenes", module: "ordenes-trabajo", prefilter: "activo = ${recordID}", presort: "inicio DESC", refField: "activo", fields: []string{"codigo", "solicitud", "tecnico", "inicio", "estado", "diagnostico", "proximoMantenimiento"}},
		},
		auxiliary: []auxiliaryPageSpec{
			{parent: "cotizaciones", handle: "item-cotizacion-registro", title: "Detalle de ítem de cotización", module: "items-cotizacion", description: "Edición de una línea económica de la cotización.", fields: []string{"cotizacion", "secuencia", "tipo", "descripcion", "cantidad", "valorUnitario", "impuestoPorcentaje", "total"}},
			{parent: "ordenes-trabajo", handle: "tarea-orden-registro", title: "Detalle de tarea de orden", module: "tareas-orden", description: "Ejecución y evidencia de una tarea del checklist técnico.", fields: []string{"orden", "secuencia", "tarea", "obligatoria", "tecnico", "estado", "inicioReal", "finReal", "resultado", "evidencia"}},
		},
	},
	"centro-contacto-co": {
		metrics: []metricSpec{
			{label: "Gestiones pendientes", description: "Contactos pendientes o con devolución de llamada programada.", module: "registros-campana", filter: "estado = 'Pendiente' OR estado = 'Callback'", metricField: "count", operation: "count"},
			{label: "Callbacks vencidos", description: "Devoluciones de llamada cuya fecha ya venció.", module: "registros-campana", filter: "estado = 'Callback' AND proximoIntento < NOW()", metricField: "count", operation: "count"},
			{label: "Oportunidades abiertas", description: "Oportunidades que continúan en gestión comercial.", module: "oportunidades", filter: "etapa != 'Venta' AND etapa != 'Perdida'", metricField: "count", operation: "count"},
			{label: "Ventas", description: "Valor de las oportunidades convertidas en venta en COP.", module: "oportunidades", filter: "etapa = 'Venta'", metricField: "valor", operation: "sum", prefix: "$ "},
		},
		reportCharts: []string{"funnel-contacto", "resultados-llamadas", "conversion-por-campana", "llamadas-por-agente"},
		reportModule: "llamadas", reportFields: []string{"contacto", "campana", "agente", "inicio", "duracionSegundos", "disposicion", "callback"},
		organizer: organizerSpec{page: "oportunidades", title: "Pipeline comercial", module: "oportunidades", labelField: "contacto", descriptionField: "producto", groupField: "etapa", filter: "etapa != 'Venta' AND etapa != 'Perdida'"},
		recordFields: []recordFieldsSpec{
			{page: "contactos-leads/contacto-registro", fields: []string{"fechaAutorizacion", "origenAutorizacion", "canalesAutorizados", "rneConsultado", "fechaConsultaRNE", "motivoNoContactar", "prioridad", "ultimaGestion"}},
		},
		queue: queuePageSpec{handle: "mi-cola", title: "Mi cola", description: "Gestiones telefónicas pendientes asignadas al usuario actual, ordenadas por prioridad y próximo intento.", module: "registros-campana", prefilter: "agente = ${userID} AND (estado = 'Pendiente' OR estado = 'Callback')", presort: "prioridad DESC, proximoIntento ASC", fields: []string{"campana", "contacto", "prioridad", "intentos", "proximoIntento", "estado", "resultadoAcumulado"}},
	},
	"soporte-renovaciones-co": {
		metrics: []metricSpec{
			{label: "Backlog abierto", description: "Casos de soporte pendientes de cierre.", module: "casos", filter: "estado != 'Cerrado' AND estado != 'Cancelado'", metricField: "count", operation: "count"},
			{label: "Casos críticos", description: "Casos críticos que todavía requieren atención.", module: "casos", filter: "prioridad = 'Crítica' AND estado != 'Cerrado'", metricField: "count", operation: "count"},
			{label: "SLA en riesgo", description: "Casos con SLA en riesgo o ya incumplido.", module: "casos", filter: "estadoSLA = 'En riesgo' OR estadoSLA = 'Incumplido'", metricField: "count", operation: "count"},
			{label: "Valor en riesgo", description: "Valor de renovaciones con riesgo medio o alto en COP.", module: "renovaciones", filter: "riesgo = 'Alto' OR riesgo = 'Medio'", metricField: "nuevoValor", operation: "sum", prefix: "$ "},
		},
		reportCharts: []string{"casos-por-prioridad", "cumplimiento-sla", "funnel-renovaciones", "valor-renovaciones"},
		reportModule: "casos", reportFields: []string{"codigo", "cliente", "contrato", "prioridad", "estado", "agente", "estadoSLA", "vencimientoResolucion"},
		organizer: organizerSpec{page: "renovaciones", title: "Pipeline de renovaciones", module: "renovaciones", labelField: "codigo", descriptionField: "proximoPaso", groupField: "etapa", filter: "etapa != 'Renovada' AND etapa != 'Perdida'"},
		recordFields: []recordFieldsSpec{
			{page: "clientes/cliente-registro", fields: []string{"fechaAutorizacion"}},
			{page: "contratos/contrato-registro", fields: []string{"diasAvisoRenovacion"}},
			{page: "casos/caso-registro", fields: []string{"casoPadre", "problemaConocido", "impacto", "urgencia", "nivelEscalamiento", "motivoEscalamiento", "causaRaiz", "categoriaResolucion", "evidencias"}},
			{page: "interacciones/interaccion-registro", fields: []string{"renovacion", "esRespuestaAgente"}},
			{page: "renovaciones/renovacion-registro", fields: []string{"motivoRiesgo", "proximoPaso", "fechaProximaGestion"}},
		},
		relations: []relationSpec{
			{page: "problemas-conocidos/problema-conocido-registro", title: "Casos relacionados", module: "casos", prefilter: "problemaConocido = ${recordID}", presort: "createdAt DESC", refField: "problemaConocido", fields: []string{"codigo", "cliente", "producto", "asunto", "prioridad", "estado", "agente", "estadoSLA"}},
		},
		auxiliary: []auxiliaryPageSpec{
			{parent: "", handle: "problemas-conocidos", title: "Problemas conocidos", module: "problemas-conocidos", description: "Base operativa de causas y soluciones compartidas para agrupar casos recurrentes.", visible: true, fields: []string{"codigo", "titulo", "producto", "estado", "impacto", "responsable", "fechaDeteccion", "fechaResolucion"}},
			{parent: "problemas-conocidos", handle: "problema-conocido-registro", title: "Detalle de problema conocido", module: "problemas-conocidos", description: "Ficha de causa raíz, solución temporal y solución definitiva de un problema recurrente.", fields: []string{"codigo", "titulo", "producto", "estado", "impacto", "causaRaiz", "solucionTemporal", "solucionDefinitiva", "responsable", "fechaDeteccion", "fechaResolucion", "observaciones"}},
		},
		queue: queuePageSpec{handle: "mi-cola", title: "Mi cola", description: "Casos abiertos asignados al agente actual, ordenados por prioridad y vencimiento de SLA.", module: "casos", prefilter: "agente = ${userID} AND estado != 'Cerrado' AND estado != 'Cancelado'", presort: "prioridad DESC, vencimientoResolucion ASC", fields: []string{"codigo", "cliente", "asunto", "prioridad", "estado", "estadoSLA", "vencimientoResolucion"}},
	},
}

type blockInfo struct {
	node    *yaml.Node
	id      string
	kind    string
	origX   int
	origY   int
	origW   int
	origH   int
	newXYWH [4]int
}

func main() {
	root := flag.String("root", filepath.Join("..", "templates"), "templates directory")
	write := flag.Bool("write", false, "materialize version-controlled template YAML (development only)")
	flag.Parse()

	if !*write {
		fmt.Fprintln(os.Stderr, "template-v2 is development-only; pass -write to update committed template YAML")
		os.Exit(2)
	}

	for _, templateID := range templateIDs {
		base := filepath.Join(*root, templateID, "provision")
		if err := normalizePages(templateID, filepath.Join(base, "pages.yaml")); err != nil {
			panic(fmt.Errorf("normalize %s pages: %w", templateID, err))
		}
		if err := generateDescriptions(templateID, filepath.Join(base, "modules.yaml"), filepath.Join(base, "descriptions-es.yaml")); err != nil {
			panic(fmt.Errorf("describe %s modules: %w", templateID, err))
		}
		if err := generateReports(templateID, filepath.Join(base, "reports.yaml"), visualConfigs[templateID]); err != nil {
			panic(fmt.Errorf("generate %s reports: %w", templateID, err))
		}
		if err := generateWorkflowExamples(filepath.Join(base, "workflows-v2.yaml"), workflowNoticeConfigs[templateID]); err != nil {
			panic(fmt.Errorf("generate %s workflow examples: %w", templateID, err))
		}
	}
}

func normalizePages(templateID, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var doc yaml.Node
	if err = yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	root := documentMap(&doc)
	pages := mapValue(root, "pages")
	if pages == nil || pages.Kind != yaml.MappingNode {
		return errors.New("pages mapping not found")
	}
	if err = enhancePages(templateID, pages); err != nil {
		return err
	}
	normalizePageMap(pages)

	var out bytes.Buffer
	out.WriteString(licenseHeader)
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err = enc.Encode(root); err != nil {
		return err
	}
	if err = enc.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func enhancePages(templateID string, pages *yaml.Node) error {
	cfg, ok := visualConfigs[templateID]
	if !ok {
		return fmt.Errorf("visual configuration not found for %s", templateID)
	}

	for _, spec := range cfg.auxiliary {
		if err := ensureAuxiliaryPage(pages, spec); err != nil {
			return err
		}
	}
	if cfg.queue.handle != "" {
		ensureQueuePage(pages, cfg.queue)
	}
	for _, spec := range cfg.recordFields {
		page := findPage(pages, spec.page)
		if page == nil {
			return fmt.Errorf("record page %s not found", spec.page)
		}
		appendBlock(page, map[string]any{
			"title":       "Gestión y control",
			"description": "Campos operativos complementarios para mantener contexto, trazabilidad y próximos pasos del registro.",
			"kind":        "Record",
			"xywh":        []int{0, nextBlockY(page), 48, 30},
			"options":     map[string]any{"fields": fieldOptions(spec.fields)},
		})
	}
	for _, spec := range cfg.relations {
		page := findPage(pages, spec.page)
		if page == nil {
			return fmt.Errorf("related-list page %s not found", spec.page)
		}
		options := map[string]any{
			"module":                     spec.module,
			"fields":                     fieldOptions(spec.fields),
			"prefilter":                  spec.prefilter,
			"presort":                    spec.presort,
			"perPage":                    10,
			"allowExport":                true,
			"showTotalCount":             true,
			"showRecordPerPageOption":    true,
			"inlineValueFiltering":       true,
			"hideRecordCloneButton":      true,
			"hideConfigureFieldsButton":  false,
			"enableRecordPageNavigation": true,
		}
		if spec.refField != "" {
			options["refField"] = spec.refField
		}
		appendBlock(page, map[string]any{
			"title":       spec.title,
			"description": "Historial relacionado que completa la vista 360° del registro actual.",
			"kind":        "RecordList",
			"xywh":        []int{0, nextBlockY(page), 48, 38},
			"options":     options,
		})
	}
	if cfg.organizer.page != "" {
		page := findPage(pages, cfg.organizer.page)
		if page == nil {
			return fmt.Errorf("organizer page %s not found", cfg.organizer.page)
		}
		appendBlock(page, map[string]any{
			"title":       cfg.organizer.title,
			"description": "Pipeline operativo agrupado por etapa; permite visualizar y actualizar el avance de cada registro.",
			"kind":        "RecordOrganizer",
			"xywh":        []int{0, nextBlockY(page), 48, 44},
			"options": map[string]any{
				"module":           cfg.organizer.module,
				"labelField":       cfg.organizer.labelField,
				"descriptionField": cfg.organizer.descriptionField,
				"groupField":       cfg.organizer.groupField,
				"filter":           cfg.organizer.filter,
				"displayOption":    "sameTab",
				"showRefresh":      true,
			},
		})
	}
	if cfg.mapBlock.page != "" {
		page := findPage(pages, cfg.mapBlock.page)
		if page == nil {
			return fmt.Errorf("map page %s not found", cfg.mapBlock.page)
		}
		appendBlock(page, map[string]any{
			"title":       cfg.mapBlock.title,
			"description": "Mapa nativo de Corteza construido con la ubicación guardada en cada registro; no requiere librerías externas.",
			"kind":        "Geometry",
			"xywh":        []int{0, nextBlockY(page), 48, 44},
			"options":     mapOptions(cfg.mapBlock),
		})
		configureMapBlock(blockByTitleAndKind(page, cfg.mapBlock.title, "Geometry"), cfg.mapBlock)
	}
	if err := ensureHomeMetrics(pages, cfg.metrics); err != nil {
		return err
	}
	ensureHomeOperationalLists(pages)
	ensureReportPage(pages, cfg)
	enhanceRecordLists(pages)
	describePages(pages)
	return nil
}

func ensureHomeMetrics(pages *yaml.Node, metrics []metricSpec) error {
	page := findPage(pages, "inicio")
	if page == nil {
		return errors.New("home page not found")
	}
	blocks := mapValue(page, "blocks")
	existing := make([]*yaml.Node, 0, len(metrics))
	if blocks != nil {
		for _, block := range blocks.Content {
			if scalarValue(mapValue(block, "kind")) == "Metric" {
				existing = append(existing, block)
			}
		}
	}
	if len(existing) == 0 {
		shiftPageBlocks(page, homeMetricHeight)
	}
	for index, metric := range metrics {
		x := (index % 4) * 12
		y := (index / 4) * homeMetricHeight
		desired := map[string]any{
			"title":       metric.label,
			"description": metricDescription(metric),
			"kind":        "Metric",
			"xywh":        []int{x, y, 12, homeMetricHeight},
			"options": map[string]any{
				"showRefresh": true,
				"metrics": []any{map[string]any{
					"label":        metric.label,
					"module":       metric.module,
					"filter":       metric.filter,
					"metricField":  metric.metricField,
					"operation":    metric.operation,
					"numberFormat": "0,0",
					"prefix":       metric.prefix,
					"suffix":       metric.suffix,
					"drillDown": map[string]any{
						"enabled": false,
					},
				}},
			},
		}
		if index >= len(existing) {
			appendBlock(page, desired)
			continue
		}
		for _, key := range []string{"title", "description", "kind", "xywh", "options"} {
			setMapValue(existing[index], key, desired[key])
		}
	}
	return nil
}

func metricDescription(metric metricSpec) string {
	return metric.description
}

func ensureHomeOperationalLists(pages *yaml.Node) {
	page := findPage(pages, "inicio")
	if page == nil {
		return
	}
	blocks := mapValue(page, "blocks")
	if blocks == nil {
		return
	}
	for _, block := range blocks.Content {
		if scalarValue(mapValue(block, "kind")) != "RecordList" {
			continue
		}
		xywh := mapValue(block, "xywh")
		if xywh == nil || len(xywh.Content) != 4 || intValue(xywh.Content[3]) >= homeOperationalListHeight {
			continue
		}
		xywh.Content[3].Value = strconv.Itoa(homeOperationalListHeight)
		xywh.Content[3].Tag = "!!int"
	}
}

func mapOptions(spec mapSpec) map[string]any {
	zoomStarting := spec.zoomStarting
	if zoomStarting == 0 {
		zoomStarting = 6
	}
	options := map[string]any{
		"center":        spec.center,
		"zoomStarting":  zoomStarting,
		"zoomMin":       4,
		"zoomMax":       18,
		"hideGeoSearch": false,
		"showRefresh":   true,
		"displayOption": "sameTab",
		"feeds": []any{map[string]any{
			"resource":      "compose:record",
			"titleField":    spec.titleField,
			"geometryField": spec.geometryField,
			"displayMarker": true,
			"options": map[string]any{
				"module":    spec.module,
				"color":     "#2563EB",
				"prefilter": spec.filter,
			},
		}},
	}
	if len(spec.bounds) > 0 {
		options["bounds"] = spec.bounds
		options["lockBounds"] = true
	}
	return options
}

func configureMapBlock(block *yaml.Node, spec mapSpec) {
	if block == nil {
		return
	}
	setMapValue(block, "description", "Mapa de los registros con ubicación geográfica válida; seleccione un marcador para abrir su ficha.")
	setMapValue(block, "options", mapOptions(spec))
}

func ensureReportPage(pages *yaml.Node, cfg visualConfig) {
	if mapValue(pages, "reportes") != nil {
		return
	}
	blocks := make([]any, 0, len(cfg.reportCharts)+1)
	layoutBlocks := make([]any, 0, len(cfg.reportCharts)+1)
	for index, chart := range cfg.reportCharts {
		x := (index % 2) * 24
		y := (index / 2) * 30
		id := index + 1
		blocks = append(blocks, map[string]any{
			"title":       humanizeTitle(chart),
			"description": "Visual ejecutivo para analizar tendencias y distribución de los registros.",
			"kind":        "Chart",
			"xywh":        []int{x, y, 24, 30},
			"options":     map[string]any{"chart": chart},
			"blockID":     id,
		})
		layoutBlocks = append(layoutBlocks, map[string]any{"blockID": id, "xywh": []int{x, y, 24, 30}})
	}
	listY := ((len(cfg.reportCharts) + 1) / 2) * 30
	listID := len(cfg.reportCharts) + 1
	blocks = append(blocks, map[string]any{
		"title":       "Detalle operativo",
		"description": "Registros que respaldan los indicadores; permite buscar, filtrar, contar y exportar el resultado.",
		"kind":        "RecordList",
		"xywh":        []int{0, listY, 48, 38},
		"options": map[string]any{
			"module":                    cfg.reportModule,
			"fields":                    fieldOptions(cfg.reportFields),
			"presort":                   "createdAt DESC",
			"perPage":                   20,
			"allowExport":               true,
			"showTotalCount":            true,
			"showRecordPerPageOption":   true,
			"inlineValueFiltering":      true,
			"hideConfigureFieldsButton": false,
		},
		"blockID": listID,
	})
	layoutBlocks = append(layoutBlocks, map[string]any{"blockID": listID, "xywh": []int{0, listY, 48, 38}})
	appendMapping(pages, "reportes", nodeFromValue(map[string]any{
		"title":       "Reportes",
		"description": "Vista ejecutiva y operativa para analizar resultados, encontrar desviaciones y exportar información.",
		"visible":     true,
		"blocks":      blocks,
		"page_layouts": map[string]any{
			"principal": map[string]any{
				"primary": true,
				"meta":    map[string]any{"title": "Diseño principal · Reportes"},
				"config":  map[string]any{"useTitle": true},
				"blocks":  layoutBlocks,
			},
		},
	}))
}

func ensureQueuePage(pages *yaml.Node, spec queuePageSpec) {
	if mapValue(pages, spec.handle) != nil {
		return
	}
	appendMapping(pages, spec.handle, makeListPage(spec.title, spec.description, spec.module, spec.prefilter, spec.presort, spec.fields, true))
}

func ensureAuxiliaryPage(pages *yaml.Node, spec auxiliaryPageSpec) error {
	target := pages
	if spec.parent != "" {
		parent := findPage(pages, spec.parent)
		if parent == nil {
			return fmt.Errorf("auxiliary parent page %s not found", spec.parent)
		}
		target = ensureMapping(parent, "children")
	}
	if mapValue(target, spec.handle) != nil {
		return nil
	}
	if spec.visible {
		appendMapping(target, spec.handle, makeListPage(spec.title, spec.description, spec.module, "", "createdAt DESC", spec.fields, true))
		return nil
	}
	block := map[string]any{
		"title":       "Información principal",
		"description": spec.description,
		"kind":        "Record",
		"xywh":        []int{0, 0, 48, maxInt(30, 10+len(spec.fields)*5)},
		"options":     map[string]any{"fields": fieldOptions(spec.fields)},
		"blockID":     1,
	}
	appendMapping(target, spec.handle, nodeFromValue(map[string]any{
		"title":       spec.title,
		"description": spec.description,
		"module":      spec.module,
		"visible":     false,
		"blocks":      []any{block},
		"page_layouts": map[string]any{
			"principal": map[string]any{
				"primary": true,
				"meta":    map[string]any{"title": "Diseño principal · " + spec.title},
				"config": map[string]any{
					"useTitle": true,
					"buttons": map[string]any{
						"new": map[string]any{"enabled": true}, "edit": map[string]any{"enabled": true},
						"submit": map[string]any{"enabled": true}, "delete": map[string]any{"enabled": true},
						"clone": map[string]any{"enabled": true}, "back": map[string]any{"enabled": true},
					},
				},
				"blocks": []any{map[string]any{"blockID": 1, "xywh": block["xywh"]}},
			},
		},
	}))
	return nil
}

func makeListPage(title, description, module, prefilter, presort string, fields []string, visible bool) *yaml.Node {
	block := map[string]any{
		"title":       title,
		"description": description,
		"kind":        "RecordList",
		"xywh":        []int{0, 0, 48, 38},
		"options": map[string]any{
			"module":                    module,
			"fields":                    fieldOptions(fields),
			"prefilter":                 prefilter,
			"presort":                   presort,
			"perPage":                   20,
			"allowExport":               true,
			"showTotalCount":            true,
			"showRecordPerPageOption":   true,
			"inlineValueFiltering":      true,
			"hideConfigureFieldsButton": false,
		},
		"blockID": 1,
	}
	return nodeFromValue(map[string]any{
		"title": title, "description": description, "visible": visible,
		"blocks": []any{block},
		"page_layouts": map[string]any{"principal": map[string]any{
			"primary": true, "meta": map[string]any{"title": "Diseño principal · " + title},
			"config": map[string]any{"useTitle": true},
			"blocks": []any{map[string]any{"blockID": 1, "xywh": []int{0, 0, 48, 38}}},
		}},
	})
}

func findPage(pages *yaml.Node, path string) *yaml.Node {
	current := pages
	var page *yaml.Node
	for index, handle := range strings.Split(path, "/") {
		page = mapValue(current, handle)
		if page == nil {
			return nil
		}
		if index < len(strings.Split(path, "/"))-1 {
			current = mapValue(page, "children")
			if current == nil {
				current = mapValue(page, "pages")
			}
			if current == nil {
				return nil
			}
		}
	}
	return page
}

func appendBlock(page *yaml.Node, block map[string]any) {
	title, _ := block["title"].(string)
	kind, _ := block["kind"].(string)
	if blockByTitleAndKind(page, title, kind) != nil {
		return
	}
	id := nextBlockID(page)
	block["blockID"] = id
	node := nodeFromValue(block)
	blocks := ensureSequence(page, "blocks")
	blocks.Content = append(blocks.Content, node)

	layouts := ensureMapping(page, "page_layouts")
	if len(layouts.Content) == 0 {
		appendMapping(layouts, "principal", nodeFromValue(map[string]any{
			"primary": true,
			"meta":    map[string]any{"title": "Diseño principal"},
			"config":  map[string]any{"useTitle": true},
			"blocks":  []any{},
		}))
	}
	xywh := block["xywh"]
	for index := 0; index+1 < len(layouts.Content); index += 2 {
		layoutBlocks := ensureSequence(layouts.Content[index+1], "blocks")
		layoutBlocks.Content = append(layoutBlocks.Content, nodeFromValue(map[string]any{"blockID": id, "xywh": xywh}))
	}
}

func blockByTitleAndKind(page *yaml.Node, title, kind string) *yaml.Node {
	blocks := mapValue(page, "blocks")
	if blocks == nil || blocks.Kind != yaml.SequenceNode {
		return nil
	}
	for _, block := range blocks.Content {
		if scalarValue(mapValue(block, "title")) == title && scalarValue(mapValue(block, "kind")) == kind {
			return block
		}
	}
	return nil
}

func blockByTitle(page *yaml.Node, title string) *yaml.Node {
	blocks := mapValue(page, "blocks")
	if blocks == nil || blocks.Kind != yaml.SequenceNode {
		return nil
	}
	for _, block := range blocks.Content {
		if scalarValue(mapValue(block, "title")) == title {
			return block
		}
	}
	return nil
}

func nextBlockID(page *yaml.Node) int {
	blocks := mapValue(page, "blocks")
	maxID := 0
	if blocks != nil {
		for _, block := range blocks.Content {
			if id := intValue(mapValue(block, "blockID")); id > maxID {
				maxID = id
			}
		}
	}
	return maxID + 1
}

func nextBlockY(page *yaml.Node) int {
	blocks := mapValue(page, "blocks")
	bottom := 0
	if blocks != nil {
		for _, block := range blocks.Content {
			xywh := mapValue(block, "xywh")
			if xywh == nil || len(xywh.Content) != 4 {
				continue
			}
			candidate := intValue(xywh.Content[1]) + intValue(xywh.Content[3])
			if candidate > bottom {
				bottom = candidate
			}
		}
	}
	return bottom
}

func shiftPageBlocks(page *yaml.Node, deltaY int) {
	shift := func(blocks *yaml.Node) {
		if blocks == nil || blocks.Kind != yaml.SequenceNode {
			return
		}
		for _, block := range blocks.Content {
			xywh := mapValue(block, "xywh")
			if xywh == nil || len(xywh.Content) != 4 {
				continue
			}
			y := intValue(xywh.Content[1]) + deltaY
			xywh.Content[1].Value = strconv.Itoa(y)
			xywh.Content[1].Tag = "!!int"
		}
	}
	shift(mapValue(page, "blocks"))
	layouts := mapValue(page, "page_layouts")
	if layouts != nil {
		for index := 0; index+1 < len(layouts.Content); index += 2 {
			shift(mapValue(layouts.Content[index+1], "blocks"))
		}
	}
}

func enhanceRecordLists(pages *yaml.Node) {
	for index := 0; index+1 < len(pages.Content); index += 2 {
		page := pages.Content[index+1]
		blocks := mapValue(page, "blocks")
		if blocks != nil {
			for _, block := range blocks.Content {
				if scalarValue(mapValue(block, "kind")) != "RecordList" {
					continue
				}
				options := ensureMapping(block, "options")
				setMapValue(options, "allowExport", true)
				setMapValue(options, "showTotalCount", true)
				setMapValue(options, "showRecordPerPageOption", true)
				setMapValue(options, "inlineValueFiltering", true)
				setMapValue(options, "hideSearch", false)
				setMapValue(options, "hideFiltering", false)
				setMapValue(options, "hideConfigureFieldsButton", false)
				setMapValue(options, "customFilterPresets", true)
			}
		}
		if children := mapValue(page, "children"); children != nil {
			enhanceRecordLists(children)
		}
		if children := mapValue(page, "pages"); children != nil {
			enhanceRecordLists(children)
		}
	}
}

func describePages(pages *yaml.Node) {
	for index := 0; index+1 < len(pages.Content); index += 2 {
		page := pages.Content[index+1]
		title := scalarValue(mapValue(page, "title"))
		if scalarValue(mapValue(page, "description")) == "" {
			setMapValue(page, "description", "Vista operativa de "+strings.ToLower(title)+". Organiza la información y sus relaciones para apoyar decisiones y tareas diarias.")
		}
		blocks := mapValue(page, "blocks")
		if blocks != nil {
			for _, block := range blocks.Content {
				if scalarValue(mapValue(block, "description")) != "" {
					continue
				}
				blockTitle := scalarValue(mapValue(block, "title"))
				kind := scalarValue(mapValue(block, "kind"))
				description := "Bloque funcional de " + strings.ToLower(blockTitle) + "."
				switch kind {
				case "RecordList":
					description = "Listado operativo de " + strings.ToLower(blockTitle) + " con búsqueda, filtros, conteo y exportación."
				case "Record":
					description = "Sección de la ficha 360° para consultar y editar " + strings.ToLower(blockTitle) + "."
				case "Chart":
					description = "Gráfico que resume " + strings.ToLower(blockTitle) + " a partir de datos actuales."
				case "Calendar":
					description = "Calendario operativo de " + strings.ToLower(blockTitle) + " en la zona horaria America/Bogota."
				}
				setMapValue(block, "description", description)
			}
		}
		if children := mapValue(page, "children"); children != nil {
			describePages(children)
		}
		if children := mapValue(page, "pages"); children != nil {
			describePages(children)
		}
	}
}

func fieldOptions(fields []string) []any {
	out := make([]any, 0, len(fields))
	for _, field := range fields {
		out = append(out, map[string]any{"name": field})
	}
	return out
}

func ensureMapping(node *yaml.Node, key string) *yaml.Node {
	if existing := mapValue(node, key); existing != nil {
		return existing
	}
	value := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendMapping(node, key, value)
	return value
}

func ensureSequence(node *yaml.Node, key string) *yaml.Node {
	if existing := mapValue(node, key); existing != nil {
		return existing
	}
	value := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	appendMapping(node, key, value)
	return value
}

func appendMapping(node *yaml.Node, key string, value *yaml.Node) {
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

func setMapValue(node *yaml.Node, key string, value any) {
	converted := nodeFromValue(value)
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			node.Content[index+1] = converted
			return
		}
	}
	appendMapping(node, key, converted)
}

func nodeFromValue(value any) *yaml.Node {
	raw, err := yaml.Marshal(value)
	if err != nil {
		panic(err)
	}
	var doc yaml.Node
	if err = yaml.Unmarshal(raw, &doc); err != nil {
		panic(err)
	}
	return documentMap(&doc)
}

func humanizeTitle(value string) string {
	value = strings.ReplaceAll(value, "-", " ")
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func normalizePageMap(pages *yaml.Node) {
	for i := 0; i+1 < len(pages.Content); i += 2 {
		page := pages.Content[i+1]
		coords := normalizeBlocks(mapValue(page, "blocks"))
		applyLayoutCoordinates(mapValue(page, "page_layouts"), coords)
		if children := mapValue(page, "children"); children != nil {
			normalizePageMap(children)
		}
		if children := mapValue(page, "pages"); children != nil {
			normalizePageMap(children)
		}
	}
}

func normalizeBlocks(blocks *yaml.Node) map[string][4]int {
	coords := make(map[string][4]int)
	if blocks == nil || blocks.Kind != yaml.SequenceNode || len(blocks.Content) == 0 {
		return coords
	}

	bb := make([]*blockInfo, 0, len(blocks.Content))
	maxRight := 0
	for index, block := range blocks.Content {
		xywh := mapValue(block, "xywh")
		if xywh == nil || xywh.Kind != yaml.SequenceNode || len(xywh.Content) != 4 {
			continue
		}
		x := intValue(xywh.Content[0])
		y := intValue(xywh.Content[1])
		w := intValue(xywh.Content[2])
		h := intValue(xywh.Content[3])
		id := scalarValue(mapValue(block, "blockID"))
		if id == "" {
			id = strconv.Itoa(index + 1)
		}
		kind := scalarValue(mapValue(block, "kind"))
		bb = append(bb, &blockInfo{node: block, id: id, kind: kind, origX: x, origY: y, origW: w, origH: h})
		if x+w > maxRight {
			maxRight = x + w
		}
	}
	if len(bb) == 0 {
		return coords
	}

	scale := 1
	if maxRight <= 12 {
		scale = 4
	}
	rows := map[int][]*blockInfo{}
	rowKeys := make([]int, 0)
	for _, b := range bb {
		if _, ok := rows[b.origY]; !ok {
			rowKeys = append(rowKeys, b.origY)
		}
		rows[b.origY] = append(rows[b.origY], b)
	}
	sort.Ints(rowKeys)

	currentY := 0
	for _, originalY := range rowKeys {
		row := rows[originalY]
		sort.SliceStable(row, func(i, j int) bool { return row[i].origX < row[j].origX })
		cursorX := 0
		rowHeight := 0
		for _, b := range row {
			height := b.origH
			if min := minimumHeightForBlock(b.node, b.kind); height < min {
				height = min
			}
			width := normalizedWidth(b, len(row), scale)
			if cursorX > 0 && cursorX+width > 48 {
				currentY += rowHeight
				cursorX = 0
				rowHeight = 0
			}
			b.newXYWH = [4]int{cursorX, currentY, width, height}
			if height > rowHeight {
				rowHeight = height
			}
			setXYWH(mapValue(b.node, "xywh"), b.newXYWH)
			coords[b.id] = b.newXYWH
			cursorX += width
			if cursorX == 48 {
				currentY += rowHeight
				cursorX = 0
				rowHeight = 0
			}
		}
		if cursorX > 0 {
			currentY += rowHeight
		}
	}
	return coords
}

func normalizedWidth(block *blockInfo, rowSize, scale int) int {
	switch block.kind {
	case "Metric":
		return 12
	case "Calendar", "Geometry", "RecordOrganizer", "Report":
		return 48
	case "Chart", "Record", "RecordList":
		if rowSize > 1 {
			return 24
		}
		return 48
	default:
		width := block.origW * scale
		if width < 12 {
			width = 12
		}
		if width > 48 {
			width = 48
		}
		return width
	}
}

func applyLayoutCoordinates(layouts *yaml.Node, coords map[string][4]int) {
	if layouts == nil || layouts.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(layouts.Content); i += 2 {
		layout := layouts.Content[i+1]
		blocks := mapValue(layout, "blocks")
		if blocks == nil || blocks.Kind != yaml.SequenceNode {
			continue
		}
		for _, block := range blocks.Content {
			id := scalarValue(mapValue(block, "blockID"))
			if xywh, ok := coords[id]; ok {
				setXYWH(mapValue(block, "xywh"), xywh)
			}
		}
	}
}

func minimumHeight(kind string) int {
	switch kind {
	case "Metric":
		return homeMetricHeight
	case "Chart":
		return 30
	case "RecordList":
		return 38
	case "Calendar", "Geometry", "RecordOrganizer", "Report":
		return 44
	case "Record":
		return 30
	case "Content", "Navigation":
		return 12
	default:
		return 24
	}
}

func minimumHeightForBlock(block *yaml.Node, kind string) int {
	height := minimumHeight(kind)
	if kind != "Record" {
		return height
	}
	options := mapValue(block, "options")
	fields := mapValue(options, "fields")
	if fields == nil || fields.Kind != yaml.SequenceNode {
		return height
	}
	needed := 10 + len(fields.Content)*5
	if needed > height {
		return needed
	}
	return height
}

func generateReports(templateID, outputPath string, cfg visualConfig) error {
	groupField := cfg.organizer.groupField
	if groupField == "" {
		groupField = "createdAt"
	}
	load := func(name string) map[string]any {
		return map[string]any{
			"meta": map[string]any{"name": "Registros de " + humanizeTitle(cfg.reportModule)},
			"step": map[string]any{
				"kind": "load",
				"load": map[string]any{
					"name":   name,
					"source": "composeRecords",
					"definition": map[string]any{
						"namespace": templateID,
						"module":    cfg.reportModule,
					},
				},
			},
		}
	}
	tableBlock := func(title, description, source string, limit int) map[string]any {
		return map[string]any{
			"title":       title,
			"description": description,
			"key":         "1",
			"kind":        "",
			"xywh":        []int{0, 0, 48, 30},
			"layout":      "horizontal",
			"elements": []any{map[string]any{
				"elementID":   "1",
				"kind":        "Table",
				"name":        title,
				"description": description,
				"options": map[string]any{
					"source": source,
					"datasources": []any{map[string]any{
						"name":   source,
						"sort":   "",
						"filter": map[string]any{},
						"paging": map[string]any{"limit": limit},
					}},
					"columns":    map[string]any{},
					"striped":    true,
					"hover":      true,
					"responsive": true,
				},
			}},
		}
	}

	reports := map[string]any{
		templateID + "-ejecutivo": map[string]any{
			"meta": map[string]any{
				"name":        "Resumen ejecutivo · " + humanizeTitle(templateID),
				"description": "Distribución ejecutiva de los registros por " + humanize(groupField) + ", útil para seguimiento de resultados y reuniones de gestión.",
			},
			"sources": []any{
				load("registros"),
				map[string]any{
					"meta": map[string]any{"name": "Resumen por " + humanize(groupField)},
					"step": map[string]any{
						"kind": "aggregate",
						"aggregate": map[string]any{
							"name":   "resumen",
							"source": "registros",
							"keys": []any{map[string]any{
								"name": groupField, "label": humanizeTitle(groupField), "def": groupField,
							}},
							"columns": []any{map[string]any{
								"name": "total", "label": "Total", "def": "count(recordID)",
							}},
						},
					},
				},
			},
			"blocks": []any{tableBlock("Resumen por "+humanize(groupField), "Cantidad de registros agrupada para identificar concentración, avance y desviaciones.", "resumen", 100)},
		},
		templateID + "-operativo": map[string]any{
			"meta": map[string]any{
				"name":        "Detalle operativo · " + humanizeTitle(templateID),
				"description": "Detalle exportable de los registros operativos que respaldan los indicadores de la plantilla.",
			},
			"sources": []any{load("detalle")},
			"blocks":  []any{tableBlock("Detalle operativo", "Tabla consultable y exportable para revisión operativa y control de calidad.", "detalle", 100)},
		},
	}
	raw, err := yaml.Marshal(map[string]any{"report": reports})
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, append([]byte(licenseHeader), raw...), 0o644)
}

func generateWorkflowExamples(outputPath string, cfg workflowNoticeConfig) error {
	trigger := func(module, event string, enabled bool, stepID int) []any {
		return []any{map[string]any{
			"enabled":      enabled,
			"stepID":       stepID,
			"resourceType": "compose:record",
			"eventType":    event,
			"constraints": []any{
				map[string]any{"name": "namespace.handle", "op": "=", "values": []any{cfg.namespace}},
				map[string]any{"name": "module.handle", "op": "=", "values": []any{module}},
			},
		}}
	}
	assignedExpression := "!isEmpty(record.values." + cfg.userField + ")"
	milestoneExpression := fmt.Sprintf("record.values.%s == %q && oldRecord.values.%s != %q", cfg.milestoneField, cfg.milestoneValue, cfg.milestoneField, cfg.milestoneValue)

	workflows := map[string]any{
		"avisar-asignacion-" + cfg.namespace: map[string]any{
			"enabled":      true,
			"trace":        false,
			"runAs":        cfg.runAs,
			"keepSessions": 20,
			"meta": map[string]any{
				"name":        "Avisar " + cfg.recordLabel + " asignado",
				"description": "Después de crear el registro, envía una notificación nativa al responsable con enlace directo. No contacta servicios externos; si el responsable está vacío termina sin error.",
			},
			"triggers": trigger(cfg.module, "afterCreate", true, 700),
			"steps": []any{
				map[string]any{"id": 700, "kind": "gateway", "ref": "excl", "meta": map[string]any{"name": "¿Tiene responsable?"}},
				map[string]any{
					"id": 701, "kind": "function", "ref": "notificationSendRecord",
					"arguments": []any{
						map[string]any{"target": "recipient", "type": "ID", "expr": "record.values." + cfg.userField},
						map[string]any{"target": "title", "type": "String", "value": "Nuevo " + cfg.recordLabel + " asignado"},
						map[string]any{"target": "description", "type": "String", "expr": fmt.Sprintf("format(%q, record.ID)", "Abre el registro y revisa su próxima acción. ID %d")},
						map[string]any{"target": "module", "type": "Handle", "value": cfg.module},
						map[string]any{"target": "namespace", "type": "Handle", "value": cfg.namespace},
						map[string]any{"target": "record", "type": "ID", "expr": "record.ID"},
						map[string]any{"target": "openMode", "type": "String", "value": "sameTab"},
						map[string]any{"target": "edit", "type": "Boolean", "value": false},
					},
					"meta": map[string]any{"name": "Notificar al responsable"},
				},
				map[string]any{"id": 702, "kind": "termination", "meta": map[string]any{"name": "Finalizar"}},
			},
			"paths": []any{
				map[string]any{"parentid": 700, "childid": 701, "expr": assignedExpression},
				map[string]any{"parentid": 700, "childid": 702, "expr": ""},
				map[string]any{"parentid": 701, "childid": 702},
			},
		},
		"ejemplo-correo-responsable-" + cfg.namespace: map[string]any{
			"enabled": false,
			"trace":   false,
			"runAs":   cfg.runAs,
			"meta": map[string]any{
				"name":        "EJEMPLO - DESACTIVADO · Correo al responsable",
				"description": "Ejemplo visual desactivado. Para habilitarlo, configure SMTP, revise remitente y contenido, pruebe en un entorno no productivo y active tanto el workflow como su trigger. Resuelve al responsable por ID y envía un correo; sin SMTP devolverá error.",
			},
			"triggers": trigger(cfg.module, "afterCreate", false, 800),
			"steps": []any{
				map[string]any{"id": 800, "kind": "gateway", "ref": "excl", "meta": map[string]any{"name": "¿Tiene responsable?"}},
				map[string]any{
					"id": 801, "kind": "function", "ref": "usersLookup",
					"arguments": []any{map[string]any{"target": "lookup", "type": "ID", "expr": "record.values." + cfg.userField}},
					"results":   []any{map[string]any{"target": "assignedUser", "expr": "user"}},
					"meta":      map[string]any{"name": "Consultar responsable"},
				},
				map[string]any{
					"id": 802, "kind": "function", "ref": "emailSend",
					"arguments": []any{
						map[string]any{"target": "to", "type": "User", "expr": "assignedUser"},
						map[string]any{"target": "subject", "type": "String", "value": "Nuevo " + cfg.recordLabel + " asignado"},
						map[string]any{"target": "plain", "type": "String", "expr": fmt.Sprintf("format(%q, record.ID)", "Tienes un nuevo registro asignado en Corteza. ID %d")},
					},
					"meta": map[string]any{"name": "Enviar correo"},
				},
				map[string]any{"id": 803, "kind": "termination", "meta": map[string]any{"name": "Finalizar"}},
			},
			"paths": []any{
				map[string]any{"parentid": 800, "childid": 801, "expr": assignedExpression},
				map[string]any{"parentid": 800, "childid": 803, "expr": ""},
				map[string]any{"parentid": 801, "childid": 802},
				map[string]any{"parentid": 802, "childid": 803},
			},
		},
		"ejemplo-slack-hito-" + cfg.namespace: map[string]any{
			"enabled": false,
			"trace":   false,
			"runAs":   cfg.runAs,
			"meta": map[string]any{
				"name":        "EJEMPLO - DESACTIVADO · Aviso de " + cfg.milestone + " en Slack",
				"description": "Ejemplo visual desactivado. Sustituya la URL example.invalid por un webhook protegido, no guarde secretos en el YAML, valide el mensaje y active workflow y trigger. Solo se ejecutaría cuando el registro alcance el hito indicado.",
			},
			"triggers": trigger(cfg.milestoneModule, "afterUpdate", false, 900),
			"steps": []any{
				map[string]any{"id": 900, "kind": "gateway", "ref": "excl", "meta": map[string]any{"name": "¿Alcanzó el hito?"}},
				map[string]any{
					"id": 901, "kind": "function", "ref": "httpRequestSend",
					"arguments": []any{
						map[string]any{"target": "url", "type": "String", "value": "https://example.invalid/configure-slack-webhook"},
						map[string]any{"target": "method", "type": "String", "value": "POST"},
						map[string]any{"target": "headerContentType", "type": "String", "value": "application/json"},
						map[string]any{"target": "timeout", "type": "Duration", "value": "10s"},
						map[string]any{"target": "body", "type": "String", "value": fmt.Sprintf("{\"text\":\"Abera: %s\"}", cfg.milestone)},
					},
					"meta": map[string]any{"name": "Publicar aviso"},
				},
				map[string]any{"id": 902, "kind": "termination", "meta": map[string]any{"name": "Finalizar"}},
			},
			"paths": []any{
				map[string]any{"parentid": 900, "childid": 901, "expr": milestoneExpression},
				map[string]any{"parentid": 900, "childid": 902, "expr": ""},
				map[string]any{"parentid": 901, "childid": 902},
			},
		},
	}
	raw, err := yaml.Marshal(map[string]any{"workflows": workflows})
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, append([]byte(licenseHeader), raw...), 0o644)
}

func generateDescriptions(templateID, modulesPath, outputPath string) error {
	raw, err := os.ReadFile(modulesPath)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err = yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	modules := mapValue(documentMap(&doc), "modules")
	if modules == nil || modules.Kind != yaml.MappingNode {
		return errors.New("modules mapping not found")
	}

	var out strings.Builder
	out.WriteString(licenseHeader)
	out.WriteString("locale:\n  es:\n")
	for i := 0; i+1 < len(modules.Content); i += 2 {
		moduleHandle := modules.Content[i].Value
		module := modules.Content[i+1]
		moduleName := scalarValue(mapValue(module, "name"))
		purpose := modulePurpose[templateID+"/"+moduleHandle]
		if purpose == "" {
			purpose = fmt.Sprintf("Registros de %s y sus relaciones dentro de la plantilla %s.", strings.ToLower(moduleName), templateID)
		}
		moduleMeta := ensureMapping(module, "meta")
		setMapValue(moduleMeta, "description", purpose)
		writeTranslation(&out, fmt.Sprintf("corteza::compose:module/%s/%s", templateID, moduleHandle), map[string]string{
			"meta.description": purpose,
		})

		fields := mapValue(module, "fields")
		if fields == nil || fields.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(fields.Content); j += 2 {
			fieldHandle := fields.Content[j].Value
			field := fields.Content[j+1]
			label := scalarValue(mapValue(field, "label"))
			kind := scalarValue(mapValue(field, "kind"))
			required := boolValue(mapValue(field, "required"))
			multi := boolValue(mapValue(field, "multi"))
			options := ensureMapping(field, "options")
			view, edit := describeField(moduleName, fieldHandle, label, kind, required, multi, options)
			description := ensureMapping(options, "description")
			setMapValue(description, "view", view)
			setMapValue(description, "edit", edit)
			writeTranslation(&out, fmt.Sprintf("corteza::compose:module-field/%s/%s/%s", templateID, moduleHandle, fieldHandle), map[string]string{
				"meta.description.view": view,
				"meta.description.edit": edit,
			})
		}
	}

	var modulesOut bytes.Buffer
	modulesOut.WriteString(licenseHeader)
	encoder := yaml.NewEncoder(&modulesOut)
	encoder.SetIndent(2)
	if err = encoder.Encode(documentMap(&doc)); err != nil {
		return err
	}
	if err = encoder.Close(); err != nil {
		return err
	}
	if err = os.WriteFile(modulesPath, modulesOut.Bytes(), 0o644); err != nil {
		return err
	}
	return os.WriteFile(outputPath, []byte(out.String()), 0o644)
}

func describeField(moduleName, handle, label, kind string, required, multi bool, options *yaml.Node) (view, edit string) {
	if label == "" {
		label = humanize(handle)
	}
	lowerLabel := strings.ToLower(label)
	requirement := " Es opcional; complételo cuando la información esté disponible."
	if required {
		requirement = " Es obligatorio para guardar un registro completo."
	}

	switch kind {
	case "Record":
		target := scalarValue(mapValue(options, "module"))
		cardinality := "un registro"
		linked := "un registro vinculado"
		if multi {
			cardinality = "varios registros"
			linked = "varios registros vinculados"
		}
		view = fmt.Sprintf("Relación con %s. Permite consultar %s en este registro de %s.", humanize(target), linked, moduleName)
		edit = fmt.Sprintf("Seleccione %s dentro del módulo %s. La relación admite %s; no escriba identificadores manualmente.%s", lowerLabel, humanize(target), cardinality, requirement)
	case "User":
		view = fmt.Sprintf("Usuario de Corteza responsable de %s en este registro de %s.", lowerLabel, moduleName)
		edit = fmt.Sprintf("Seleccione un usuario activo con el rol operativo correspondiente.%s", requirement)
	case "DateTime":
		if boolValue(mapValue(options, "onlyDate")) {
			view = fmt.Sprintf("Fecha de %s para este registro de %s.", lowerLabel, moduleName)
			edit = fmt.Sprintf("Indique %s en formato de fecha; no agregue una hora.%s", lowerLabel, requirement)
		} else {
			view = fmt.Sprintf("Fecha y hora de %s para este registro de %s, interpretada en America/Bogota.", lowerLabel, moduleName)
			edit = fmt.Sprintf("Indique %s usando la zona horaria America/Bogota.%s", lowerLabel, requirement)
		}
	case "Number":
		prefix := scalarValue(mapValue(options, "prefix"))
		if strings.Contains(prefix, "$") || strings.Contains(strings.ToLower(label), "valor") || strings.Contains(strings.ToLower(label), "precio") || strings.Contains(strings.ToLower(label), "presupuesto") {
			view = fmt.Sprintf("%s expresado en pesos colombianos (COP) para este registro de %s.", label, moduleName)
			edit = fmt.Sprintf("Ingrese %s en COP, sin símbolos ni separadores manuales.%s", lowerLabel, requirement)
		} else {
			view = fmt.Sprintf("Valor numérico de %s para este registro de %s.", lowerLabel, moduleName)
			edit = fmt.Sprintf("Ingrese %s como número.%s", lowerLabel, requirement)
		}
	case "Bool":
		view = fmt.Sprintf("Indica si %s aplica a este registro de %s.", lowerLabel, moduleName)
		edit = fmt.Sprintf("Marque Sí únicamente cuando %s esté confirmado; en caso contrario seleccione No.%s", lowerLabel, requirement)
	case "Select":
		choices := selectChoices(options)
		view = fmt.Sprintf("Clasificación de %s para este registro de %s.", lowerLabel, moduleName)
		if multi {
			edit = fmt.Sprintf("Seleccione uno o varios valores para %s", lowerLabel)
		} else {
			edit = fmt.Sprintf("Seleccione un valor para %s", lowerLabel)
		}
		if choices != "" {
			edit += " entre: " + choices
		}
		edit += "." + requirement
	case "File":
		view = fmt.Sprintf("Archivos asociados con %s en este registro de %s.", lowerLabel, moduleName)
		if multi {
			edit = fmt.Sprintf("Adjunte uno o varios archivos pertinentes para %s. No cargue credenciales ni información innecesaria.%s", lowerLabel, requirement)
		} else {
			edit = fmt.Sprintf("Adjunte el archivo pertinente para %s. No cargue credenciales ni información innecesaria.%s", lowerLabel, requirement)
		}
	case "Geometry":
		view = fmt.Sprintf("Ubicación geográfica de %s, utilizada por los mapas y la planeación operativa.", moduleName)
		edit = fmt.Sprintf("Busque o marque en el mapa la ubicación correspondiente a %s. Verifique el punto antes de guardar.%s", lowerLabel, requirement)
	case "Email":
		view = fmt.Sprintf("Correo electrónico de contacto para este registro de %s.", moduleName)
		edit = fmt.Sprintf("Ingrese una dirección de correo válida, por ejemplo nombre@empresa.com.%s", requirement)
	default:
		view = fmt.Sprintf("El campo «%s» conserva información operativa del registro en %s.", label, moduleName)
		edit = fmt.Sprintf("Ingrese %s con información clara, concreta y verificable; evite abreviaturas ambiguas.%s", lowerLabel, requirement)
	}

	if automaticFields[handle] {
		edit = "No editar manualmente. Este valor es administrado por workflows o integraciones de la plantilla."
	}
	if handle == "externalID" {
		edit = "No editar manualmente salvo durante una integración controlada. Debe contener el identificador estable del registro en el sistema externo."
	}
	if strings.Contains(strings.ToLower(handle), "autoriza") {
		view = "Evidencia de que la persona autorizó el tratamiento o contacto conforme a la finalidad informada."
		edit = "Marque Sí únicamente después de obtener una autorización previa, informada y verificable; registre también su fecha y origen cuando estén disponibles."
	}
	if strings.Contains(strings.ToLower(handle), "telefono") {
		edit = "Ingrese el número de contacto en formato internacional, preferiblemente +57 seguido de diez dígitos para Colombia." + requirement
	}
	if strings.Contains(strings.ToLower(handle), "correo") {
		edit = "Ingrese una dirección de correo válida, por ejemplo nombre@empresa.com." + requirement
	}
	if strings.EqualFold(handle, "fechaAutorizacion") {
		edit = "Registre la fecha y hora en que se obtuvo la autorización verificable, usando America/Bogota." + requirement
	}
	return
}

func selectChoices(options *yaml.Node) string {
	choicesNode := mapValue(options, "options")
	if choicesNode == nil || choicesNode.Kind != yaml.SequenceNode || len(choicesNode.Content) > 10 {
		return ""
	}
	choices := make([]string, 0, len(choicesNode.Content))
	for _, choice := range choicesNode.Content {
		text := scalarValue(mapValue(choice, "text"))
		if text == "" {
			text = scalarValue(mapValue(choice, "value"))
		}
		if text != "" {
			choices = append(choices, text)
		}
	}
	return strings.Join(choices, ", ")
}

func writeTranslation(out *strings.Builder, resource string, values map[string]string) {
	out.WriteString("    ")
	out.WriteString(strconv.Quote(resource))
	out.WriteString(":\n")
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out.WriteString("      ")
		out.WriteString(key)
		out.WriteString(": ")
		out.WriteString(strconv.Quote(values[key]))
		out.WriteByte('\n')
	}
}

func documentMap(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

func mapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func scalarValue(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	return node.Value
}

func intValue(node *yaml.Node) int {
	v, _ := strconv.Atoi(scalarValue(node))
	return v
}

func boolValue(node *yaml.Node) bool {
	v, _ := strconv.ParseBool(scalarValue(node))
	return v
}

func setXYWH(node *yaml.Node, values [4]int) {
	if node == nil || node.Kind != yaml.SequenceNode || len(node.Content) != 4 {
		return
	}
	for i, value := range values {
		node.Content[i].Kind = yaml.ScalarNode
		node.Content[i].Tag = "!!int"
		node.Content[i].Value = strconv.Itoa(value)
	}
}

func humanize(value string) string {
	if value == "" {
		return "el registro relacionado"
	}
	value = strings.ReplaceAll(value, "-", " ")
	var out strings.Builder
	for i, r := range value {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
	}
	return strings.ToLower(out.String())
}
