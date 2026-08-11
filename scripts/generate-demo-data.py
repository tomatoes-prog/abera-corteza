#!/usr/bin/env python3

# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

"""Generate deterministic, synthetic Envoy datasets for the Abera showroom."""

from __future__ import annotations

import csv
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

import yaml


ROOT = Path(__file__).resolve().parents[1]
ANCHOR = datetime(2026, 7, 31, 12, 0, tzinfo=timezone.utc)
ADVISORS = ["asesora-demo", "asesor-demo"]
CITIES = [
    ("Bogotá D.C.", "Bogotá"),
    ("Antioquia", "Medellín"),
    ("Valle del Cauca", "Cali"),
    ("Atlántico", "Barranquilla"),
    ("Bolívar", "Cartagena"),
    ("Santander", "Bucaramanga"),
]
FIRST_NAMES = [
    "Sofía", "Mateo", "Valentina", "Santiago", "Mariana", "Sebastián",
    "Isabella", "Nicolás", "Gabriela", "Samuel", "Laura", "Daniel",
]
LAST_NAMES = [
    "García", "Rodríguez", "Martínez", "López", "González", "Hernández",
    "Pérez", "Sánchez", "Ramírez", "Torres", "Gómez", "Restrepo",
]


def iso(value: datetime) -> str:
    return value.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def day(value: datetime) -> str:
    return value.strftime("%Y-%m-%d")


def person(index: int) -> tuple[str, str]:
    return FIRST_NAMES[index % len(FIRST_NAMES)], LAST_NAMES[(index * 3) % len(LAST_NAMES)]


def ref(prefix: str, index: int) -> str:
    return f"{prefix}-{index:03d}"


def row(record_id: str, **values: Any) -> dict[str, Any]:
    return {"ID": record_id, **{key: value for key, value in values.items() if value is not None}}


def inmobiliaria() -> dict[str, list[dict[str, Any]]]:
    properties: list[dict[str, Any]] = []
    property_types = ["Apartamento", "Casa", "Oficina", "Local", "Finca", "Bodega"]
    operations = ["Venta", "Arriendo", "Venta o arriendo"]
    property_states = ["Disponible"] * 25 + ["Reservado"] * 5 + ["Arrendado", "Vendido", "Inactivo"] * 2
    for i in range(1, 37):
        department, city = CITIES[(i - 1) % len(CITIES)]
        operation = operations[(i - 1) % len(operations)]
        properties.append(row(
            ref("inmueble", i),
            codigo=f"INM-DEMO-{i:03d}",
            nombre=f"{property_types[(i - 1) % len(property_types)]} en {city}",
            descripcion="Inmueble sintético preparado para la demostración comercial de Abera.",
            tipoOperacion=operation,
            tipoInmueble=property_types[(i - 1) % len(property_types)],
            estado=property_states[(i - 1) % len(property_states)],
            departamento=department,
            ciudad=city,
            direccion=f"Carrera {(i * 7) % 90 + 1} # {(i * 11) % 70 + 1}-{(i * 13) % 99:02d}",
            precioVenta=220_000_000 + i * 31_000_000 if operation != "Arriendo" else None,
            canonArrendamiento=1_400_000 + i * 115_000 if operation != "Venta" else None,
            administracion=180_000 + i * 17_000,
            area=45 + (i * 9) % 220,
            habitaciones=1 + i % 5,
            banos=1 + i % 4,
            parqueaderos=i % 3,
            estrato=str(2 + i % 5),
            propietarioNombre=f"Propietario sintético {i}",
            propietarioTelefono=f"+57 310 555 {i:04d}",
        ))

    leads: list[dict[str, Any]] = []
    lead_states = (
        ["Nuevo"] * 18 + ["En contacto"] * 18 + ["Calificado"] * 18
        + ["Cita agendada"] * 16 + ["Negociación"] * 12
        + ["Ganado"] * 8 + ["Perdido"] * 6 + ["Descartado"] * 4
    )
    for i in range(1, 101):
        first, last = person(i)
        department, city = CITIES[(i - 1) % len(CITIES)]
        interested = [ref("inmueble", (i * step) % 36 + 1) for step in (1, 3, 5)][: 1 + i % 3]
        leads.append(row(
            ref("lead", i),
            nombres=first,
            apellidos=last,
            telefono=f"+57 300 700 {i:04d}",
            correo=f"lead.demo.{i:03d}@abera.local",
            autorizaTratamientoDatos="1",
            tipoOperacion=["Venta", "Arriendo", "Venta o arriendo"][i % 3],
            tiposInmueble=["Apartamento", "Casa"] if i % 2 else ["Oficina", "Local"],
            departamentoBusqueda=department,
            ciudadBusqueda=city,
            presupuestoMinimo=120_000_000 + i * 2_500_000,
            presupuestoMaximo=280_000_000 + i * 7_500_000,
            habitacionesMinimas=1 + i % 3,
            banosMinimos=1 + i % 2,
            inmueblesInteres=interested,
            estado=lead_states[i - 1],
            responsable=None if i % 10 == 0 else ADVISORS[i % 2],
            fechaAsignacion=iso(ANCHOR - timedelta(days=i % 25)),
            proximoContacto=iso(ANCHOR + timedelta(hours=6 + i % 96)),
            notas="Lead sintético con necesidad y presupuesto definidos.",
        ))

    appointments = [
        row(
            ref("cita", i),
            asunto=f"Visita inmobiliaria #{i}",
            lead=ref("lead", (i * 2) % 100 + 1),
            asesor=ADVISORS[i % 2],
            inmuebles=[ref("inmueble", i % 36 + 1), ref("inmueble", (i + 7) % 36 + 1)],
            inicio=iso(ANCHOR + timedelta(days=(i % 30) - 12, hours=9 + i % 7)),
            fin=iso(ANCHOR + timedelta(days=(i % 30) - 12, hours=10 + i % 7)),
            tipo=["Visita", "Llamada", "Virtual", "Oficina"][i % 4],
            estado=["Pendiente", "Confirmada", "Completada", "Cancelada", "Reprogramada"][i % 5],
            puntoEncuentro="Recepción o portería del inmueble.",
            resultado="Interés confirmado y siguiente paso registrado.",
            observaciones="Cita sintética relacionada con varios inmuebles.",
        )
        for i in range(1, 61)
    ]
    activities = [
        row(
            ref("actividad", i),
            lead=ref("lead", i % 100 + 1),
            inmueble=ref("inmueble", i % 36 + 1),
            asesor=ADVISORS[i % 2],
            tipo=["Llamada", "WhatsApp", "Correo", "Nota", "Seguimiento"][i % 5],
            fecha=iso(ANCHOR - timedelta(hours=i * 3)),
            resultado="Contacto efectivo y necesidad actualizada.",
            proximaAccion=iso(ANCHOR + timedelta(days=1 + i % 15)),
            observaciones="Actividad comercial sintética.",
        )
        for i in range(1, 101)
    ]
    negotiations = [
        row(
            ref("negociacion", i),
            lead=ref("lead", i),
            inmueble=ref("inmueble", i % 36 + 1),
            asesor=ADVISORS[i % 2],
            tipoOperacion=["Venta", "Arriendo"][i % 2],
            etapa=["Calificación", "Oferta", "Documentación", "Cierre"][i % 4],
            valorPublicado=250_000_000 + i * 12_000_000,
            valorOfrecido=235_000_000 + i * 11_500_000,
            probabilidad=[25, 45, 70, 90][i % 4],
            fechaEstimadaCierre=day(ANCHOR + timedelta(days=7 + i)),
            estado=["Abierta", "Abierta", "Ganada", "Perdida"][i % 4],
            motivoPerdida="Condiciones económicas" if i % 4 == 3 else None,
            observaciones="Negociación sintética.",
        )
        for i in range(1, 36)
    ]
    return {
        "inmuebles": properties,
        "leads": leads,
        "citas": appointments,
        "actividades": activities,
        "negociaciones": negotiations,
    }


def automotriz() -> dict[str, list[dict[str, Any]]]:
    brands = ["Renault", "Chevrolet", "Mazda", "Toyota", "Kia", "Volkswagen"]
    vehicles = [
        row(
            ref("vehiculo", i),
            codigo=f"AUTO-DEMO-{i:03d}",
            vin=f"9ABERADEMO{i:08d}",
            placa=f"DMO{i:03d}",
            marca=brands[i % len(brands)],
            modelo=f"Modelo {1 + i % 8}",
            version=["Comfort", "Touring", "Premium"][i % 3],
            anio=2022 + i % 5,
            condicion=["Nuevo", "Usado"][i % 2],
            kilometraje=0 if i % 2 else 8_000 + i * 530,
            color=["Blanco", "Gris", "Azul", "Rojo"][i % 4],
            precio=62_000_000 + i * 3_850_000,
            sede=CITIES[i % len(CITIES)][1],
            fechaIngreso=day(ANCHOR - timedelta(days=i * 3)),
            estado=["Disponible", "Disponible", "Reservado", "Vendido"][i % 4],
            proximoMantenimiento=day(ANCHOR + timedelta(days=30 + i * 4)),
            externalID=f"DMS-VEH-{i:03d}",
            proveedorExterno="DMS sintético",
            estadoIntegracion="Local",
        )
        for i in range(1, 41)
    ]
    prospects = []
    funnel = ["Nuevo", "Contactado", "Calificado", "Prueba de manejo", "Cotización", "Negociación", "Vendido", "Perdido"]
    for i in range(1, 101):
        first, last = person(i)
        prospects.append(row(
            ref("auto-prospecto", i),
            nombres=first,
            apellidos=last,
            telefono=f"+57 301 610 {i:04d}",
            correo=f"auto.demo.{i:03d}@abera.local",
            autorizaTratamientoDatos="1",
            ciudad=CITIES[i % len(CITIES)][1],
            presupuestoMinimo=55_000_000 + i * 300_000,
            presupuestoMaximo=90_000_000 + i * 1_500_000,
            plazoCompra=["Inmediato", "Menos de 30 días", "Entre 1 y 3 meses", "Más de 3 meses"][i % 4],
            vehiculoActual=f"Vehículo usado {i % 12}",
            requiereFinanciacion=str(i % 2),
            vehiculosInteres=[ref("vehiculo", i % 40 + 1), ref("vehiculo", (i + 9) % 40 + 1)],
            origen=["Sitio web", "Sala de ventas", "Referido", "Redes sociales"][i % 4],
            estado=funnel[i % len(funnel)],
            asesor=None if i % 12 == 0 else ADVISORS[i % 2],
            fechaAsignacion=iso(ANCHOR - timedelta(days=i % 20)),
            proximoContacto=iso(ANCHOR + timedelta(hours=i % 96)),
            notas="Prospecto automotriz sintético.",
            externalID=f"CRM-AUTO-{i:03d}",
            proveedorExterno="Formulario demo",
        ))
    tests = [
        row(
            ref("prueba", i),
            asunto=f"Prueba de manejo #{i}",
            prospecto=ref("auto-prospecto", i % 100 + 1),
            vehiculo=ref("vehiculo", i % 40 + 1),
            asesor=ADVISORS[i % 2],
            inicio=iso(ANCHOR + timedelta(days=(i % 25) - 10, hours=8 + i % 8)),
            fin=iso(ANCHOR + timedelta(days=(i % 25) - 10, hours=9 + i % 8)),
            sede=CITIES[i % len(CITIES)][1],
            estado=["Pendiente", "Confirmada", "Completada", "Cancelada"][i % 4],
            resultado=["Interesado", "Requiere otra opción", "Sin interés"][i % 3],
            siguienteAccion="Preparar cotización y alternativa de financiación.",
            observaciones="Prueba sintética.",
        )
        for i in range(1, 61)
    ]
    opportunities = [
        row(
            ref("auto-oportunidad", i),
            prospecto=ref("auto-prospecto", i),
            vehiculo=ref("vehiculo", i % 40 + 1),
            asesor=ADVISORS[i % 2],
            etapa=["Cotización", "Financiación", "Negociación", "Documentación", "Vendido", "Perdido"][i % 6],
            valorPublicado=70_000_000 + i * 2_400_000,
            valorOfertado=68_000_000 + i * 2_250_000,
            requiereFinanciacion=str(i % 2),
            valorFinanciado=35_000_000 + i * 500_000,
            recibeRetoma=str((i + 1) % 2),
            descripcionRetoma=f"Retoma sintética #{i}",
            probabilidad=[20, 40, 60, 80, 100, 0][i % 6],
            fechaEstimadaCierre=day(ANCHOR + timedelta(days=5 + i)),
            motivoPerdida="Precio o financiación" if i % 6 == 5 else None,
            observaciones="Oportunidad automotriz sintética.",
        )
        for i in range(1, 36)
    ]
    orders = [
        row(
            ref("orden-taller", i),
            codigo=f"OT-DEMO-{i:03d}",
            cliente=ref("auto-prospecto", i % 100 + 1),
            vehiculo=ref("vehiculo", i % 40 + 1),
            coordinador="supervisora-demo",
            tecnico="tecnica-demo",
            inicio=iso(ANCHOR + timedelta(days=(i % 28) - 14, hours=7 + i % 9)),
            fin=iso(ANCHOR + timedelta(days=(i % 28) - 14, hours=9 + i % 9)),
            tipoServicio=["Mantenimiento preventivo", "Inspección", "Reparación", "Garantía"][i % 4],
            estado=["Programada", "En ejecución", "Completada", "Cancelada"][i % 4],
            solicitud="Revisión general del vehículo.",
            diagnostico="Diagnóstico sintético documentado.",
            valorEstimado=350_000 + i * 45_000,
            valorFinal=330_000 + i * 47_000,
            proximoMantenimiento=day(ANCHOR + timedelta(days=90 + i)),
            observaciones="Orden de taller sintética.",
            externalID=f"DMS-OT-{i:03d}",
        )
        for i in range(1, 51)
    ]
    return {
        "vehiculos": vehicles,
        "prospectos": prospects,
        "pruebas-manejo": tests,
        "oportunidades": opportunities,
        "ordenes-servicio": orders,
    }


def educacion() -> dict[str, list[dict[str, Any]]]:
    programs = [
        row(
            ref("programa", i),
            codigo=f"EDU-PROG-{i:02d}",
            nombre=["Administración", "Ingeniería de Sistemas", "Mercadeo", "Diseño", "Contaduría"][i % 5] + f" {i}",
            nivel=["Técnico", "Tecnólogo", "Pregrado", "Especialización", "Maestría", "Educación continua"][i % 6],
            modalidad=["Presencial", "Virtual", "Híbrida"][i % 3],
            sede=CITIES[i % len(CITIES)][1],
            duracion=f"{4 + i % 7} semestres",
            valor=2_800_000 + i * 460_000,
            periodo="2026-2",
            cupos=35 + i * 4,
            metaMatriculas=25 + i * 3,
            estado="Activo",
            descripcion="Programa académico sintético.",
            externalID=f"LMS-PROG-{i:02d}",
        )
        for i in range(1, 11)
    ]
    prospects = []
    states = ["Interesado", "Contactado", "Calificado", "Solicitud iniciada", "Solicitud completa", "Admitido", "Matriculado", "No matriculado"]
    for i in range(1, 101):
        first, last = person(i)
        prospects.append(row(
            ref("edu-prospecto", i),
            nombres=first,
            apellidos=last,
            documento=f"10{i:08d}",
            telefono=f"+57 302 620 {i:04d}",
            correo=f"estudiante.demo.{i:03d}@abera.local",
            autorizaTratamientoDatos="1",
            acudienteNombre=f"Acudiente sintético {i}" if i % 4 == 0 else None,
            acudienteTelefono=f"+57 303 630 {i:04d}" if i % 4 == 0 else None,
            ciudad=CITIES[i % len(CITIES)][1],
            nivelInteres=["Técnico", "Tecnólogo", "Pregrado", "Posgrado"][i % 4],
            modalidadPreferida=["Presencial", "Virtual", "Híbrida"][i % 3],
            programasInteres=[ref("programa", i % 10 + 1), ref("programa", (i + 3) % 10 + 1)],
            origen=["Sitio web", "Feria educativa", "Referido", "Redes sociales"][i % 4],
            estado=states[i % len(states)],
            asesor=None if i % 11 == 0 else ADVISORS[i % 2],
            fechaAsignacion=iso(ANCHOR - timedelta(days=i % 30)),
            proximoContacto=iso(ANCHOR + timedelta(hours=i % 120)),
            notas="Prospecto educativo sintético.",
            externalID=f"EDU-LEAD-{i:03d}",
            proveedorExterno="Formulario demo",
            estadoIntegracion="Local",
        ))
    requests = [
        row(
            ref("solicitud-edu", i),
            codigo=f"SOL-DEMO-{i:03d}",
            prospecto=ref("edu-prospecto", i),
            programa=ref("programa", i % 10 + 1),
            periodo="2026-2",
            estado=["Borrador", "Enviada", "En revisión", "Incompleta", "Admitida", "No admitida"][i % 6],
            completitud=[30, 55, 75, 90, 100][i % 5],
            documentosVerificados=str(i % 3 == 0).lower(),
            revisor="supervisora-demo",
            puntaje=55 + i % 46,
            decision=["Lista de espera", "Admitir", "No admitir"][i % 3],
            fechaEnvio=iso(ANCHOR - timedelta(days=i % 25)),
            fechaDecision=iso(ANCHOR - timedelta(days=i % 8)) if i % 3 else None,
            observaciones="Solicitud sintética con seguimiento de completitud.",
            externalID=f"ADM-SOL-{i:03d}",
        )
        for i in range(1, 71)
    ]
    appointments = [
        row(
            ref("cita-admision", i),
            asunto=f"Orientación de admisiones #{i}",
            prospecto=ref("edu-prospecto", i % 100 + 1),
            asesor=ADVISORS[i % 2],
            programa=ref("programa", i % 10 + 1),
            tipo=["Llamada", "Entrevista", "Visita a sede", "Sesión virtual"][i % 4],
            inicio=iso(ANCHOR + timedelta(days=(i % 24) - 10, hours=8 + i % 9)),
            fin=iso(ANCHOR + timedelta(days=(i % 24) - 10, hours=9 + i % 9)),
            estado=["Pendiente", "Confirmada", "Completada", "No asistió"][i % 4],
            resultado="Interés y requisitos confirmados.",
            siguienteAccion="Completar documentos.",
            observaciones="Cita de admisión sintética.",
        )
        for i in range(1, 51)
    ]
    enrollments = [
        row(
            ref("matricula", i),
            codigo=f"MAT-DEMO-{i:03d}",
            solicitud=ref("solicitud-edu", i),
            estudiante=ref("edu-prospecto", i),
            programa=ref("programa", i % 10 + 1),
            periodo="2026-2",
            valor=3_200_000 + i * 190_000,
            descuentoBeca=(i % 4) * 250_000,
            estadoPago=["Pendiente", "Parcial", "Pagado"][i % 3],
            estado=["Preparada", "Confirmada", "Cancelada"][i % 3],
            fechaMatricula=day(ANCHOR - timedelta(days=i % 20)),
            observaciones="Matrícula sintética.",
            externalID=f"SIS-MAT-{i:03d}",
        )
        for i in range(1, 31)
    ]
    return {
        "programas": programs,
        "prospectos": prospects,
        "solicitudes": requests,
        "citas-admision": appointments,
        "matriculas": enrollments,
    }


def servicios() -> dict[str, list[dict[str, Any]]]:
    customers = []
    for i in range(1, 101):
        first, last = person(i)
        department, city = CITIES[i % len(CITIES)]
        customers.append(row(
            ref("cliente-servicio", i),
            tipo=["Prospecto", "Cliente"][i % 2],
            tipoPersona=["Persona", "Empresa"][i % 2],
            nombre=f"{first} {last}" if i % 2 else f"Empresa Técnica Demo {i} S.A.S.",
            identificacion=f"900{i:06d}",
            contacto=f"{first} {last}",
            telefono=f"+57 304 640 {i:04d}",
            correo=f"servicio.demo.{i:03d}@abera.local",
            autorizaTratamientoDatos="1",
            departamento=department,
            ciudad=city,
            direccion=f"Calle {i % 90 + 1} # {i % 60 + 1}-20",
            latitud=4.5 + (i % 20) / 100,
            longitud=-74.2 + (i % 20) / 100,
            segmento=["Hogar", "Pyme", "Corporativo"][i % 3],
            estadoComercial=["Nuevo", "Diagnóstico", "Cotización", "Cliente"][i % 4],
            responsable=ADVISORS[i % 2],
            proximoContacto=iso(ANCHOR + timedelta(days=i % 30)),
            notas="Cliente sintético de servicios técnicos.",
            externalID=f"FSM-CLI-{i:03d}",
            proveedorExterno="Formulario demo",
        ))
    assets = [
        row(
            ref("activo", i),
            codigo=f"ACT-DEMO-{i:03d}",
            cliente=ref("cliente-servicio", i % 100 + 1),
            tipoEquipo=["Aire acondicionado", "Caldera", "Panel solar", "Equipo industrial"][i % 4],
            marca=["Samsung", "LG", "Bosch", "Siemens"][i % 4],
            modelo=f"Modelo-{i % 15:02d}",
            serial=f"SN-DEMO-{i:06d}",
            fechaInstalacion=day(ANCHOR - timedelta(days=120 + i * 3)),
            garantiaHasta=day(ANCHOR + timedelta(days=90 + i)),
            estado=["Operativo", "Requiere mantenimiento", "Fuera de servicio"][i % 3],
            proximoMantenimiento=day(ANCHOR + timedelta(days=i % 90)),
            ubicacion=CITIES[i % len(CITIES)][1],
            observaciones="Activo sintético.",
            externalID=f"IOT-ACT-{i:03d}",
        )
        for i in range(1, 61)
    ]
    requests = [
        row(
            ref("solicitud-servicio", i),
            codigo=f"ST-SOL-{i:03d}",
            cliente=ref("cliente-servicio", i),
            activo=ref("activo", i % 60 + 1),
            canal=["Teléfono", "WhatsApp", "Correo", "Web"][i % 4],
            tipoServicio=["Instalación", "Mantenimiento", "Reparación", "Diagnóstico remoto"][i % 4],
            prioridad=["Baja", "Media", "Alta", "Crítica"][i % 4],
            descripcion="Solicitud sintética con alcance técnico documentado.",
            estado=["Recibida", "En diagnóstico", "Cotizada", "Aceptada", "Programada", "Ejecutada", "Cerrada"][i % 7],
            responsable="supervisora-demo",
            vencimientoSLA=iso(ANCHOR + timedelta(hours=(i % 72) - 18)),
            slaIncumplido=str(i % 9 == 0).lower(),
            externalID=f"FSM-SOL-{i:03d}",
            proveedorExterno="Portal demo",
            estadoIntegracion="Local",
        )
        for i in range(1, 101)
    ]
    quotes = [
        row(
            ref("cotizacion", i),
            codigo=f"COT-DEMO-{i:03d}",
            solicitud=ref("solicitud-servicio", i),
            cliente=ref("cliente-servicio", i),
            alcance="Diagnóstico, repuestos y mano de obra.",
            valorAntesImpuestos=450_000 + i * 52_000,
            impuestos=85_500 + i * 9_880,
            total=535_500 + i * 61_880,
            fechaVencimiento=day(ANCHOR + timedelta(days=5 + i % 20)),
            estado=["Borrador", "Enviada", "Aceptada", "Rechazada", "Vencida"][i % 5],
            fechaAceptacion=day(ANCHOR - timedelta(days=i % 8)) if i % 5 == 2 else None,
            observaciones="Cotización sintética.",
            externalID=f"ERP-COT-{i:03d}",
        )
        for i in range(1, 71)
    ]
    orders = [
        row(
            ref("orden-trabajo", i),
            codigo=f"OTR-DEMO-{i:03d}",
            solicitud=ref("solicitud-servicio", i),
            cotizacion=ref("cotizacion", i),
            cliente=ref("cliente-servicio", i),
            activo=ref("activo", i % 60 + 1),
            tecnico="tecnica-demo",
            direccion=f"Calle de servicio {i}, {CITIES[i % len(CITIES)][1]}",
            inicio=iso(ANCHOR + timedelta(days=(i % 24) - 12, hours=7 + i % 8)),
            fin=iso(ANCHOR + timedelta(days=(i % 24) - 12, hours=9 + i % 8)),
            estado=["Programada", "En camino", "En ejecución", "Completada", "Cancelada"][i % 5],
            diagnostico="Diagnóstico sintético.",
            solucion="Servicio ejecutado según procedimiento.",
            resolucionPrimeraVisita=str(i % 4 != 0).lower(),
            valorFinal=520_000 + i * 65_000,
            proximoMantenimiento=day(ANCHOR + timedelta(days=90 + i)),
            externalID=f"FSM-OT-{i:03d}",
        )
        for i in range(1, 51)
    ]
    return {
        "clientes-prospectos": customers,
        "activos": assets,
        "solicitudes": requests,
        "cotizaciones": quotes,
        "ordenes-trabajo": orders,
    }


def contacto() -> dict[str, list[dict[str, Any]]]:
    contacts = []
    states = ["Cargado", "Intentado", "Contactado", "Calificado", "Oportunidad", "Venta", "No interesado"]
    for i in range(1, 101):
        first, last = person(i)
        contacts.append(row(
            ref("contacto", i),
            nombres=first,
            apellidos=last,
            identificacion=f"CC{i:08d}",
            telefonoPrincipal=f"+57 305 650 {i:04d}",
            telefonoAlterno=f"+57 315 750 {i:04d}",
            correo=f"contacto.demo.{i:03d}@abera.local",
            ciudad=CITIES[i % len(CITIES)][1],
            autorizaTratamientoDatos="1",
            autorizaContacto="1",
            noLlamar=str(i % 23 == 0).lower(),
            productoInteres=["Plan Básico", "Plan Premium", "Servicio Empresarial"][i % 3],
            presupuesto=600_000 + i * 42_000,
            estadoComercial=states[i % len(states)],
            responsable="agente-demo",
            totalIntentos=i % 6,
            proximaLlamada=iso(ANCHOR + timedelta(hours=i % 96)),
            notas="Contacto sintético para televentas.",
            externalID=f"DIALER-CON-{i:03d}",
            proveedorExterno="Marcador demo",
            estadoIntegracion="Local",
        ))
    campaigns = [
        row(
            ref("campana", i),
            codigo=f"CALL-CAMP-{i:02d}",
            nombre=["Venta cruzada", "Prospectos digitales", "Recuperación", "Renovación"][i - 1],
            objetivo=["Venta", "Venta", "Recuperación", "Venta adicional"][i - 1],
            producto=["Plan Premium", "Servicio Empresarial", "Plan Regreso", "Renovación Anual"][i - 1],
            guion="Saludo, descubrimiento de necesidad, presentación de valor y cierre.",
            prioridad=["Alta", "Media", "Alta", "Media"][i - 1],
            fechaInicio=day(ANCHOR - timedelta(days=10)),
            fechaFin=day(ANCHOR + timedelta(days=30)),
            horaInicio="08:00",
            horaFin="18:00",
            metaContactos=120,
            metaVentas=35,
            maximoIntentos=5,
            estado="Activa",
            externalID=f"DIALER-CAMP-{i:02d}",
        )
        for i in range(1, 5)
    ]
    dial_records = [
        row(
            ref("registro-campana", i),
            codigo=f"CALL-REG-{i:03d}",
            campana=ref("campana", i % 4 + 1),
            contacto=ref("contacto", i),
            agente="agente-demo",
            prioridad=["Baja", "Media", "Alta"][i % 3],
            intentos=i % 5,
            proximoIntento=iso(ANCHOR + timedelta(hours=i % 72)),
            estado=["Pendiente", "En gestión", "Callback", "Contactado", "Calificado", "Cerrado"][i % 6],
            ultimoResultado=["Pendiente", "No contesta", "Callback", "Contactado", "Calificado", "No interesado"][i % 6],
            fechaAsignacion=iso(ANCHOR - timedelta(days=i % 20)),
            externalID=f"DIALER-REG-{i:03d}",
        )
        for i in range(1, 101)
    ]
    calls = [
        row(
            ref("llamada", i),
            registroCampana=ref("registro-campana", (i - 1) % 100 + 1),
            contacto=ref("contacto", (i - 1) % 100 + 1),
            campana=ref("campana", (i - 1) % 4 + 1),
            agente="agente-demo",
            direccion="Saliente",
            inicio=iso(ANCHOR - timedelta(minutes=i * 17)),
            fin=iso(ANCHOR - timedelta(minutes=i * 17) + timedelta(seconds=40 + i % 420)),
            duracionSegundos=40 + i % 420,
            disposicion=["No contesta", "Ocupado", "Callback", "Contactado", "Calificado", "No interesado"][i % 6],
            resumen="Llamada sintética con disposición comercial.",
            callback=iso(ANCHOR + timedelta(days=1 + i % 7)),
            externalID=f"CALL-DEMO-{i:04d}",
            proveedorExterno="Telefonía demo",
            grabacionURL=f"https://example.invalid/recordings/{i}",
            transcripcionURL=f"https://example.invalid/transcripts/{i}",
            estadoIntegracion="Local",
        )
        for i in range(1, 101)
    ]
    opportunities = [
        row(
            ref("contacto-oportunidad", i),
            codigo=f"CALL-OPP-{i:03d}",
            contacto=ref("contacto", i),
            campana=ref("campana", i % 4 + 1),
            closer="supervisora-demo",
            producto=["Plan Premium", "Servicio Empresarial", "Plan Regreso", "Renovación Anual"][i % 4],
            etapa=["Calificada", "Contacto comercial", "Propuesta", "Negociación", "Venta"][i % 5],
            valor=900_000 + i * 280_000,
            probabilidad=[25, 40, 60, 80, 100][i % 5],
            fechaEstimadaCierre=day(ANCHOR + timedelta(days=5 + i)),
            observaciones="Oportunidad sintética originada en llamada.",
        )
        for i in range(1, 41)
    ]
    return {
        "contactos-leads": contacts,
        "campanas": campaigns,
        "registros-campana": dial_records,
        "llamadas": calls,
        "oportunidades": opportunities,
    }


def soporte() -> dict[str, list[dict[str, Any]]]:
    customers = []
    for i in range(1, 61):
        first, last = person(i)
        customers.append(row(
            ref("cliente-soporte", i),
            codigo=f"SUP-CLI-{i:03d}",
            tipo=["Persona", "Empresa", "Empresa"][i % 3],
            nombre=f"{first} {last}" if i % 3 == 0 else f"Cliente Empresarial {i} S.A.S.",
            identificacion=f"901{i:06d}",
            contactoPrincipal=f"{first} {last}",
            telefono=f"+57 306 660 {i:04d}",
            correo=f"soporte.demo.{i:03d}@abera.local",
            autorizaTratamientoDatos="1",
            segmento=["Emprendimiento", "Pyme", "Corporativo"][i % 3],
            planActual=["Básico", "Estándar", "Premium"][i % 3],
            responsable="supervisora-demo",
            valorRecurrente=350_000 + i * 95_000,
            salud=["Saludable", "Saludable", "Atención", "En riesgo"][i % 4],
            satisfaccion=2 + i % 4,
            riesgo=["Bajo", "Bajo", "Medio", "Alto"][i % 4],
            notas="Cliente sintético de soporte.",
            externalID=f"CSM-CLIENT-{i:03d}",
            proveedorExterno="CSM demo",
        ))
    contracts = [
        row(
            ref("contrato", i),
            codigo=f"SUP-CON-{i:03d}",
            cliente=ref("cliente-soporte", (i - 1) % 60 + 1),
            producto=["Plataforma CRM", "Mesa de ayuda", "Analítica", "Automatización"][i % 4],
            nivelSoporte=["Básico", "Estándar", "Premium"][i % 3],
            fechaInicio=day(ANCHOR - timedelta(days=250 + i % 90)),
            fechaFin=day(ANCHOR + timedelta(days=30 + i % 180)),
            renovacionAutomatica=str(i % 2),
            valor=4_800_000 + i * 320_000,
            slaRespuestaHoras=[24, 8, 2][i % 3],
            slaResolucionHoras=[72, 24, 8][i % 3],
            estado="Activo",
            observaciones="Contrato sintético.",
            externalID=f"BILL-CON-{i:03d}",
        )
        for i in range(1, 81)
    ]
    cases = [
        row(
            ref("caso", i),
            codigo=f"SUP-CASO-{i:03d}",
            cliente=ref("cliente-soporte", (i - 1) % 60 + 1),
            contrato=ref("contrato", (i - 1) % 80 + 1),
            contacto=f"Contacto sintético {i}",
            producto=["Plataforma CRM", "Mesa de ayuda", "Analítica", "Automatización"][i % 4],
            canal=["Correo", "Teléfono", "Chat", "WhatsApp", "Portal"][i % 5],
            categoria=["Acceso", "Configuración", "Error", "Consulta", "Integración"][i % 5],
            asunto=f"Caso de demostración #{i}",
            descripcion="Descripción sintética de una solicitud de soporte.",
            prioridad=["Baja", "Media", "Alta", "Crítica"][i % 4],
            estado=["Nuevo", "Asignado", "En progreso", "Esperando cliente", "Resuelto", "Cerrado"][i % 6],
            agente="agente-demo",
            primeraRespuesta=iso(ANCHOR - timedelta(hours=i % 120) + timedelta(hours=1 + i % 4)),
            fechaResolucion=iso(ANCHOR - timedelta(hours=i % 30)) if i % 6 >= 4 else None,
            vencimientoRespuesta=iso(ANCHOR - timedelta(hours=i % 120) + timedelta(hours=8)),
            vencimientoResolucion=iso(ANCHOR - timedelta(hours=i % 120) + timedelta(hours=24)),
            estadoSLA=["En tiempo", "En tiempo", "En riesgo", "Incumplido"][i % 4],
            slaIncumplido=str(i % 4 == 3).lower(),
            resolucion="Resolución sintética documentada." if i % 6 >= 4 else None,
            resolucionPrimerContacto=str(i % 3 != 0).lower(),
            csat=1 + i % 5,
            externalID=f"HELPDESK-{i:03d}",
            proveedorExterno="Canal demo",
            estadoIntegracion="Local",
        )
        for i in range(1, 101)
    ]
    interactions = [
        row(
            ref("interaccion", i),
            caso=ref("caso", (i - 1) % 100 + 1),
            cliente=ref("cliente-soporte", (i - 1) % 60 + 1),
            canal=["Correo", "Teléfono", "Chat", "WhatsApp", "Nota interna"][i % 5],
            agente="agente-demo",
            fecha=iso(ANCHOR - timedelta(minutes=i * 19)),
            tipo=["Respuesta al cliente", "Mensaje del cliente", "Nota interna"][i % 3],
            resumen="Interacción sintética de soporte.",
            externalID=f"SUP-INT-{i:04d}",
            proveedorExterno="Canal demo",
        )
        for i in range(1, 101)
    ]
    renewals = [
        row(
            ref("renovacion", i),
            codigo=f"REN-DEMO-{i:03d}",
            cliente=ref("cliente-soporte", (i - 1) % 60 + 1),
            contrato=ref("contrato", i),
            responsable="supervisora-demo",
            fechaGestion=day(ANCHOR + timedelta(days=i % 30 - 15)),
            fechaVencimiento=day(ANCHOR + timedelta(days=30 + i % 180)),
            etapa=["Próxima a vencer", "Contactada", "Propuesta", "Comprometida", "Renovada", "Perdida"][i % 6],
            valorActual=4_800_000 + i * 320_000,
            nuevoValor=5_200_000 + i * 350_000,
            probabilidad=[60, 65, 75, 90, 100, 0][i % 6],
            riesgo=["Bajo", "Medio", "Alto"][i % 3],
            ventaAdicional=str(i % 2),
            valorExpansion=600_000 + i * 50_000 if i % 2 else 0,
            motivoPerdida="El cliente eligió otra solución." if i % 6 == 5 else None,
            observaciones="Renovación sintética.",
        )
        for i in range(1, 41)
    ]
    return {
        "clientes": customers,
        "contratos": contracts,
        "casos": cases,
        "interacciones": interactions,
        "renovaciones": renewals,
    }


def scalar(value: Any) -> str:
    if isinstance(value, list):
        return "[" + ",".join(str(item) for item in value) + "]"
    if isinstance(value, bool):
        return "1" if value else "0"
    return "" if value is None else str(value)


def write_dataset(template_id: str, datasets: dict[str, list[dict[str, Any]]]) -> None:
    module_path = ROOT / "templates" / template_id / "provision" / "modules.yaml"
    schema = yaml.safe_load(module_path.read_text(encoding="utf-8"))
    modules = schema["modules"]
    output = ROOT / "templates" / template_id / "demo" / "provision"
    output.mkdir(parents=True, exist_ok=True)

    sources: list[dict[str, Any]] = []
    for module_handle, rows in datasets.items():
        if module_handle not in modules:
            raise ValueError(f"{template_id}: unknown module {module_handle}")
        if not rows:
            raise ValueError(f"{template_id}/{module_handle}: empty dataset")

        fields = modules[module_handle].get("fields", {})
        columns = ["ID"]
        for item in rows:
            for key in item:
                if key != "ID" and key not in columns:
                    columns.append(key)
        unknown = [column for column in columns if column != "ID" and column not in fields]
        if unknown:
            raise ValueError(f"{template_id}/{module_handle}: unknown fields {unknown}")
        for field_name, field in fields.items():
            if field.get("required") and not all(scalar(item.get(field_name)) for item in rows):
                raise ValueError(f"{template_id}/{module_handle}: required field {field_name} is missing")

        filename = f"{template_id}-{module_handle}.csv"
        with (output / filename).open("w", encoding="utf-8", newline="") as stream:
            writer = csv.DictWriter(stream, fieldnames=columns, extrasaction="ignore")
            writer.writeheader()
            for item in rows:
                writer.writerow({column: scalar(item.get(column)) for column in columns})

        references: dict[str, str] = {
            "namespace": template_id,
            "module": module_handle,
        }
        for field_name, field in fields.items():
            if field.get("kind") != "Record" or field_name not in columns:
                continue
            target = field.get("options", {}).get("module")
            if target not in datasets:
                raise ValueError(f"{template_id}/{module_handle}.{field_name}: missing target dataset {target}")
            references[f"{field_name}.module"] = target
            references[f"{field_name}.datasource"] = target

        sources.append({
            "source": filename,
            "key": "ID",
            "references": references,
            "defaultable": True,
            "map": {"ID": {"column": "ID", "skip": True}},
        })

    yaml_text = yaml.safe_dump(
        {"record_sources": sources},
        allow_unicode=True,
        sort_keys=False,
        default_flow_style=False,
        width=120,
    )
    (output / "records.yaml").write_text(
        "# Copyright 2026 Abera/Corteza contributors\n"
        "# Licensed under the Apache License, Version 2.0.\n"
        "# Generated by scripts/generate-demo-data.py; all records are synthetic.\n\n"
        + yaml_text,
        encoding="utf-8",
        newline="\n",
    )


def main() -> None:
    datasets = {
        "inmobiliaria-co": inmobiliaria(),
        "automotriz-co": automotriz(),
        "admisiones-educativas-co": educacion(),
        "servicios-tecnicos-co": servicios(),
        "centro-contacto-co": contacto(),
        "soporte-renovaciones-co": soporte(),
    }
    for template_id, template_data in datasets.items():
        write_dataset(template_id, template_data)
        total = sum(len(rows) for rows in template_data.values())
        print(f"{template_id}: {total} registros sintéticos")


if __name__ == "__main__":
    main()
