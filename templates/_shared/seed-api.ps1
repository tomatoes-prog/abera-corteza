# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

Set-StrictMode -Version Latest

function Get-AberaQueryParameter {
    param([string]$Uri, [string]$Name)

    foreach ($item in ([uri]$Uri).Query.TrimStart('?').Split('&')) {
        $parts = $item.Split('=', 2)
        if ($parts.Count -eq 2 -and $parts[0] -eq $Name) {
            return [uri]::UnescapeDataString($parts[1])
        }
    }

    return $null
}

function Invoke-AberaWithoutRedirect {
    param(
        [Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [string]$Uri
    )

    $response = Invoke-WebRequest `
        -UseBasicParsing `
        -WebSession $Session `
        -Uri $Uri `
        -MaximumRedirection 0 `
        -ErrorAction SilentlyContinue

    if (-not $response) {
        throw "No se pudo seguir la redirección OAuth desde $Uri."
    }

    return [pscustomobject]@{
        Location = [string]$response.Headers['Location']
    }
}

function Get-AberaAccessToken {
    param(
        [string]$BaseUrl,
        [string]$ClientId,
        [string]$ClientSecret,
        [string]$Email,
        [string]$Password
    )

    $session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    $loginPage = Invoke-WebRequest `
        -UseBasicParsing `
        -WebSession $session `
        -Uri "$BaseUrl/auth/login"
    $tokenMatch = [regex]::Match(
        $loginPage.Content,
        'name="same-site-authenticity-token" value="([^"]+)"'
    )
    if (-not $tokenMatch.Success) {
        throw 'No se encontró el token de autenticidad del formulario.'
    }

    Invoke-WebRequest `
        -UseBasicParsing `
        -WebSession $session `
        -Method Post `
        -Uri "$BaseUrl/auth/login" `
        -Body @{
            'same-site-authenticity-token' = $tokenMatch.Groups[1].Value
            email = $Email
            password = $Password
        } | Out-Null

    if (
        @(
            $session.Cookies.GetCookies([uri]"$BaseUrl/auth/") |
                Where-Object { $_.Name -eq 'session' }
        ).Count -eq 0
    ) {
        throw 'Corteza rechazó las credenciales del usuario.'
    }

    $redirectUri = "$BaseUrl/auth/callback"
    $state = 'abera-seed-' + [guid]::NewGuid().ToString('N')
    $currentUri = (
        "$BaseUrl/auth/oauth2/authorize" +
        "?client_id=$([uri]::EscapeDataString($ClientId))" +
        "&redirect_uri=$([uri]::EscapeDataString($redirectUri))" +
        '&response_mode=query&response_type=code&scope=profile%20api' +
        "&state=$state"
    )
    $authorizationCode = $null

    for ($redirect = 0; $redirect -lt 8; $redirect++) {
        $step = Invoke-AberaWithoutRedirect -Session $session -Uri $currentUri
        if ([string]::IsNullOrWhiteSpace($step.Location)) {
            throw "OAuth se detuvo sin redirección en $currentUri."
        }

        $nextUri = [uri]::new([uri]$BaseUrl, $step.Location).AbsoluteUri
        if ($nextUri.StartsWith($redirectUri, [StringComparison]::OrdinalIgnoreCase)) {
            if ((Get-AberaQueryParameter -Uri $nextUri -Name state) -ne $state) {
                throw 'El estado devuelto por OAuth no coincide.'
            }
            $authorizationCode = Get-AberaQueryParameter -Uri $nextUri -Name code
            break
        }

        $currentUri = $nextUri
    }

    if ([string]::IsNullOrWhiteSpace($authorizationCode)) {
        throw 'OAuth no devolvió un código de autorización.'
    }

    $tokenResponse = Invoke-RestMethod `
        -Method Post `
        -Uri "$BaseUrl/auth/oauth2/token" `
        -ContentType 'application/x-www-form-urlencoded' `
        -Body @{
            grant_type = 'authorization_code'
            client_id = $ClientId
            client_secret = $ClientSecret
            code = $authorizationCode
            redirect_uri = $redirectUri
        }

    if ([string]::IsNullOrWhiteSpace($tokenResponse.access_token)) {
        throw 'OAuth no devolvió un token de acceso.'
    }

    return [string]$tokenResponse.access_token
}

function ConvertTo-AberaRecordValues {
    param([System.Collections.IDictionary]$Values)

    $output = @()
    foreach ($name in $Values.Keys) {
        $rawValue = $Values[$name]
        if ($null -eq $rawValue) {
            continue
        }

        $items = if (
            $rawValue -is [System.Collections.IEnumerable] -and
            $rawValue -isnot [string]
        ) {
            @($rawValue)
        } else {
            @($rawValue)
        }

        foreach ($item in $items) {
            if ($null -eq $item -or [string]::IsNullOrWhiteSpace([string]$item)) {
                continue
            }

            $text = if ($item -is [System.IFormattable]) {
                $item.ToString($null, [Globalization.CultureInfo]::InvariantCulture)
            } else {
                [string]$item
            }
            $output += [pscustomobject]@{ name = [string]$name; value = $text }
        }
    }

    return @($output)
}

function Get-AberaRecordValues {
    param($Record, [string]$Name)

    if ($Record.values -is [System.Collections.IDictionary]) {
        if ($null -eq $Record.values[$Name]) {
            return @()
        }
        return @($Record.values[$Name])
    }

    return @(
        $Record.values |
            Where-Object { $_.name -eq $Name } |
            ForEach-Object { $_.value }
    )
}

function Get-AberaRecordValue {
    param($Record, [string]$Name)

    return @(Get-AberaRecordValues -Record $Record -Name $Name) |
        Select-Object -First 1
}

function Invoke-AberaComposeGet {
    param([string]$Path)

    return Invoke-RestMethod `
        -Method Get `
        -Headers $script:AberaSeed.Headers `
        -Uri ($script:AberaSeed.ComposeApi + $Path)
}

function Get-AberaModuleRecords {
    param([string]$ModuleHandle)

    $moduleId = $script:AberaSeed.ModuleIds[$ModuleHandle]
    $response = Invoke-AberaComposeGet `
        -Path "/namespace/$($script:AberaSeed.NamespaceId)/module/$moduleId/record/?limit=500&incTotal=true"
    return @($response.response.set)
}

function New-AberaRecord {
    param(
        [string]$ModuleHandle,
        [System.Collections.IDictionary]$Values
    )

    $moduleId = $script:AberaSeed.ModuleIds[$ModuleHandle]
    $payload = @{
        values = @(ConvertTo-AberaRecordValues -Values $Values)
    } | ConvertTo-Json -Depth 12 -Compress

    $response = Invoke-RestMethod `
        -Method Post `
        -Headers $script:AberaSeed.Headers `
        -ContentType 'application/json' `
        -Uri "$($script:AberaSeed.ComposeApi)/namespace/$($script:AberaSeed.NamespaceId)/module/$moduleId/record/" `
        -Body ([Text.Encoding]::UTF8.GetBytes($payload))

    if (-not $response.PSObject.Properties['response']) {
        throw "La API no devolvió el registro creado para $ModuleHandle."
    }
    return $response.response
}

function Ensure-AberaRecords {
    param(
        [string]$ModuleHandle,
        [string]$KeyField,
        [array]$Items
    )

    Write-Output "Preparando $($Items.Count) registros de $ModuleHandle..."
    $existing = @(Get-AberaModuleRecords -ModuleHandle $ModuleHandle)
    $byKey = @{}
    foreach ($record in $existing) {
        $key = [string](Get-AberaRecordValue -Record $record -Name $KeyField)
        if (-not [string]::IsNullOrWhiteSpace($key)) {
            $byKey[$key] = $record
        }
    }

    $records = @()
    $position = 0
    foreach ($item in $Items) {
        $position++
        $key = [string]$item[$KeyField]
        if ($byKey.ContainsKey($key)) {
            $record = $byKey[$key]
        } else {
            $record = New-AberaRecord -ModuleHandle $ModuleHandle -Values $item
            $byKey[$key] = $record
        }
        $records += $record

        if ($position % 50 -eq 0 -or $position -eq $Items.Count) {
            Write-Output "  $ModuleHandle listos: $position/$($Items.Count)"
        }
    }

    return @($records)
}

function Get-AberaDemoNames {
    return @(
        'Sofía Martínez', 'Mateo Rodríguez', 'Valentina Gómez',
        'Santiago López', 'Mariana González', 'Sebastián Hernández',
        'Isabella Pérez', 'Nicolás Sánchez', 'Gabriela Ramírez',
        'Samuel Torres', 'Laura Flores', 'Daniel Rivera',
        'Natalia García', 'Alejandro Díaz', 'Carolina Reyes',
        'David Morales', 'Juliana Ortiz', 'Tomás Gutiérrez',
        'Paula Ruiz', 'Julián Mendoza'
    )
}

function Get-AberaDemoCities {
    return @(
        'Bogotá', 'Medellín', 'Cali', 'Barranquilla', 'Cartagena',
        'Bucaramanga', 'Pereira', 'Manizales', 'Ibagué', 'Villavicencio'
    )
}

function New-AberaAutomotiveData {
    $now = (Get-Date).ToUniversalTime()
    $names = Get-AberaDemoNames
    $cities = Get-AberaDemoCities
    $brands = @('Renault', 'Chevrolet', 'Mazda', 'Toyota', 'Kia', 'Nissan', 'Ford', 'Volkswagen')
    $models = @('Duster', 'Tracker', 'CX-30', 'Corolla Cross', 'Sportage', 'Kicks', 'Escape', 'Taos')
    $stages = @('Nuevo', 'Contactado', 'Calificado', 'Prueba de manejo', 'Cotización', 'Negociación', 'Vendido', 'Perdido')

    $vehicleItems = @()
    for ($i = 1; $i -le 40; $i++) {
        $vehicleItems += [ordered]@{
            codigo = 'AUTO-DEMO-{0:D3}' -f $i
            vin = '9ABERA{0:D11}' -f $i
            marca = $brands[($i - 1) % $brands.Count]
            modelo = $models[($i - 1) % $models.Count]
            version = 'Versión ' + (1 + ($i % 4))
            anio = 2022 + ($i % 5)
            condicion = $(if ($i % 3 -eq 0) { 'Usado' } else { 'Nuevo' })
            kilometraje = $(if ($i % 3 -eq 0) { 12000 + ($i * 850) } else { 15 })
            color = @('Blanco', 'Gris', 'Negro', 'Rojo', 'Azul')[$i % 5]
            precio = 72000000 + (($i * 6700000) % 180000000)
            sede = $cities[$i % 5]
            fechaIngreso = $now.AddDays(-$i).ToString('yyyy-MM-dd')
            estado = $(if ($i -le 32) { 'Disponible' } elseif ($i -le 36) { 'Reservado' } else { 'Vendido' })
            externalID = 'DMS-AUTO-{0:D3}' -f $i
            proveedorExterno = 'Demo DMS'
            estadoIntegracion = 'Local'
        }
    }
    $vehicles = Ensure-AberaRecords vehiculos codigo $vehicleItems

    $prospectItems = @()
    for ($i = 1; $i -le 100; $i++) {
        $name = $names[($i - 1) % $names.Count].Split(' ', 2)
        $interest = @(
            [string]$vehicles[($i - 1) % $vehicles.Count].recordID,
            [string]$vehicles[($i + 7) % $vehicles.Count].recordID
        ) | Select-Object -Unique
        $prospectItems += [ordered]@{
            nombres = $name[0]
            apellidos = $name[1]
            telefono = '+57 300{0:D7}' -f $i
            correo = 'auto.demo.{0:D3}@abera.local' -f $i
            autorizaTratamientoDatos = '1'
            ciudad = $cities[($i - 1) % $cities.Count]
            presupuestoMinimo = 60000000 + (($i * 4000000) % 100000000)
            presupuestoMaximo = 110000000 + (($i * 7000000) % 180000000)
            plazoCompra = @('Inmediato', 'Menos de 30 días', 'Entre 1 y 3 meses', 'Más de 3 meses')[$i % 4]
            vehiculoActual = $(if ($i % 2 -eq 0) { 'Vehículo usado para posible retoma' } else { 'Sin vehículo actual' })
            requiereFinanciacion = [string]($i % 2)
            vehiculosInteres = $interest
            origen = @('Sitio web', 'Sala de ventas', 'Referido', 'Redes sociales', 'Llamada')[$i % 5]
            estado = $stages[($i - 1) % $stages.Count]
            asesor = $script:AberaSeed.ActorId
            fechaAsignacion = $now.ToString('yyyy-MM-ddTHH:mm:ssZ')
            proximoContacto = $now.AddDays(1 + ($i % 20)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            notas = "Prospecto automotriz de demostración #$i."
            externalID = 'AUTO-LEAD-{0:D3}' -f $i
        }
    }
    $prospects = Ensure-AberaRecords prospectos correo $prospectItems

    $testItems = @()
    for ($i = 1; $i -le 60; $i++) {
        $start = $now.AddDays(1 + ($i % 25)).AddHours(8 + ($i % 8))
        $testItems += [ordered]@{
            asunto = 'AUTO-DEMO-PRUEBA-{0:D3}' -f $i
            prospecto = [string]$prospects[($i - 1) % $prospects.Count].recordID
            vehiculo = [string]$vehicles[($i - 1) % $vehicles.Count].recordID
            asesor = $script:AberaSeed.ActorId
            inicio = $start.ToString('yyyy-MM-ddTHH:mm:ssZ')
            fin = $start.AddHours(1).ToString('yyyy-MM-ddTHH:mm:ssZ')
            sede = $cities[$i % 5]
            estado = $(if ($i -le 35) { 'Completada' } else { 'Confirmada' })
            resultado = $(if ($i % 3 -eq 0) { 'Requiere otra opción' } else { 'Interesado' })
            siguienteAccion = 'Seguimiento comercial'
        }
    }
    $tests = Ensure-AberaRecords pruebas-manejo asunto $testItems

    $opportunityItems = @()
    for ($i = 1; $i -le 35; $i++) {
        $opportunityItems += [ordered]@{
            prospecto = [string]$prospects[$i - 1].recordID
            vehiculo = [string]$vehicles[($i - 1) % $vehicles.Count].recordID
            asesor = $script:AberaSeed.ActorId
            etapa = @('Cotización', 'Financiación', 'Negociación', 'Documentación', 'Vendido')[$i % 5]
            valorPublicado = 90000000 + ($i * 3500000)
            valorOfertado = 86000000 + ($i * 3400000)
            requiereFinanciacion = [string]($i % 2)
            probabilidad = @(25, 45, 65, 85, 100)[$i % 5]
            fechaEstimadaCierre = $now.AddDays(10 + $i).ToString('yyyy-MM-dd')
            observaciones = 'AUTO-DEMO-OPP-{0:D3}' -f $i
        }
    }
    $opportunities = Ensure-AberaRecords oportunidades observaciones $opportunityItems

    $serviceItems = @()
    for ($i = 1; $i -le 50; $i++) {
        $start = $now.AddDays(($i % 30) - 10).AddHours(8 + ($i % 8))
        $serviceItems += [ordered]@{
            codigo = 'AUTO-DEMO-OS-{0:D3}' -f $i
            cliente = [string]$prospects[($i - 1) % $prospects.Count].recordID
            vehiculo = [string]$vehicles[($i - 1) % $vehicles.Count].recordID
            coordinador = $script:AberaSeed.ActorId
            tecnico = $script:AberaSeed.ActorId
            inicio = $start.ToString('yyyy-MM-ddTHH:mm:ssZ')
            fin = $start.AddHours(3).ToString('yyyy-MM-ddTHH:mm:ssZ')
            tipoServicio = @('Mantenimiento preventivo', 'Reparación', 'Garantía', 'Inspección')[$i % 4]
            estado = @('Programada', 'Recibida', 'En ejecución', 'Completada', 'Entregada')[$i % 5]
            solicitud = 'Servicio automotriz de demostración.'
            diagnostico = 'Diagnóstico sintético para validación.'
            valorEstimado = 250000 + ($i * 45000)
            valorFinal = 260000 + ($i * 47000)
            proximoMantenimiento = $now.AddDays(120 + $i).ToString('yyyy-MM-dd')
        }
    }
    $services = Ensure-AberaRecords ordenes-servicio codigo $serviceItems

    return [ordered]@{
        prospectos = $prospects.Count
        vehiculos = $vehicles.Count
        pruebas = $tests.Count
        oportunidades = $opportunities.Count
        ordenesServicio = $services.Count
    }
}

function New-AberaEducationData {
    $now = (Get-Date).ToUniversalTime()
    $names = Get-AberaDemoNames
    $cities = Get-AberaDemoCities
    $programNames = @(
        'Administración de Empresas', 'Ingeniería de Sistemas',
        'Diseño Gráfico', 'Contaduría Pública', 'Mercadeo Digital',
        'Gestión Logística', 'Analítica de Datos', 'Talento Humano',
        'Seguridad y Salud en el Trabajo', 'Gerencia de Proyectos'
    )

    $programItems = @()
    for ($i = 1; $i -le 10; $i++) {
        $programItems += [ordered]@{
            codigo = 'EDU-DEMO-PROG-{0:D2}' -f $i
            nombre = $programNames[$i - 1]
            nivel = @('Técnico', 'Tecnólogo', 'Pregrado', 'Especialización')[$i % 4]
            modalidad = @('Presencial', 'Virtual', 'Híbrida')[$i % 3]
            sede = @('Bogotá', 'Medellín', 'Cali')[$i % 3]
            duracion = "$(2 + ($i % 8)) semestres"
            valor = 2800000 + ($i * 420000)
            periodo = '2026-2'
            cupos = 35 + ($i * 3)
            metaMatriculas = 25 + ($i * 2)
            estado = 'Activo'
            descripcion = 'Programa académico sintético para demostración.'
            externalID = 'SIS-PROG-{0:D2}' -f $i
        }
    }
    $programs = Ensure-AberaRecords programas codigo $programItems

    $states = @('Interesado', 'Contactado', 'Calificado', 'Solicitud iniciada', 'Solicitud completa', 'Admitido', 'Matriculado', 'No matriculado')
    $prospectItems = @()
    for ($i = 1; $i -le 100; $i++) {
        $name = $names[($i - 1) % $names.Count].Split(' ', 2)
        $prospectItems += [ordered]@{
            nombres = $name[0]
            apellidos = $name[1]
            documento = '10{0:D8}' -f $i
            telefono = '+57 301{0:D7}' -f $i
            correo = 'edu.demo.{0:D3}@abera.local' -f $i
            autorizaTratamientoDatos = '1'
            ciudad = $cities[$i % $cities.Count]
            nivelInteres = @('Técnico', 'Tecnólogo', 'Pregrado', 'Posgrado')[$i % 4]
            modalidadPreferida = @('Presencial', 'Virtual', 'Híbrida')[$i % 3]
            programasInteres = @(
                [string]$programs[($i - 1) % 10].recordID,
                [string]$programs[($i + 2) % 10].recordID
            )
            origen = @('Sitio web', 'Feria educativa', 'Referido', 'Redes sociales', 'Llamada')[$i % 5]
            estado = $states[($i - 1) % $states.Count]
            asesor = $script:AberaSeed.ActorId
            fechaAsignacion = $now.ToString('yyyy-MM-ddTHH:mm:ssZ')
            proximoContacto = $now.AddDays(1 + ($i % 20)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            notas = "Prospecto educativo de demostración #$i."
            externalID = 'PORTAL-EDU-{0:D3}' -f $i
        }
    }
    $prospects = Ensure-AberaRecords prospectos correo $prospectItems

    $applicationItems = @()
    for ($i = 1; $i -le 70; $i++) {
        $applicationItems += [ordered]@{
            codigo = 'EDU-DEMO-SOL-{0:D3}' -f $i
            prospecto = [string]$prospects[$i - 1].recordID
            programa = [string]$programs[($i - 1) % 10].recordID
            periodo = '2026-2'
            estado = @('Borrador', 'En revisión', 'Incompleta', 'Lista para decisión', 'Admitida', 'No admitida')[$i % 6]
            completitud = @(35, 65, 80, 100)[$i % 4]
            documentosVerificados = [string]($i % 2)
            revisor = $script:AberaSeed.ActorId
            puntaje = 60 + ($i % 41)
            decision = $(if ($i % 6 -eq 4) { 'Admitir' } elseif ($i % 6 -eq 5) { 'No admitir' } else { $null })
            fechaEnvio = $now.AddDays(-($i % 30)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            observaciones = 'Solicitud educativa sintética.'
            externalID = 'SIS-SOL-{0:D3}' -f $i
        }
    }
    $applications = Ensure-AberaRecords solicitudes codigo $applicationItems

    $appointmentItems = @()
    for ($i = 1; $i -le 50; $i++) {
        $start = $now.AddDays(($i % 25) - 5).AddHours(8 + ($i % 8))
        $appointmentItems += [ordered]@{
            asunto = 'EDU-DEMO-CITA-{0:D3}' -f $i
            prospecto = [string]$prospects[$i - 1].recordID
            asesor = $script:AberaSeed.ActorId
            programa = [string]$programs[($i - 1) % 10].recordID
            tipo = @('Llamada', 'Entrevista', 'Visita a sede', 'Sesión virtual')[$i % 4]
            inicio = $start.ToString('yyyy-MM-ddTHH:mm:ssZ')
            fin = $start.AddHours(1).ToString('yyyy-MM-ddTHH:mm:ssZ')
            estado = @('Pendiente', 'Confirmada', 'Completada', 'No asistió')[$i % 4]
            resultado = 'Resultado de demostración.'
            siguienteAccion = 'Continuar proceso de admisión.'
        }
    }
    $appointments = Ensure-AberaRecords citas-admision asunto $appointmentItems

    $enrollmentItems = @()
    for ($i = 1; $i -le 30; $i++) {
        $enrollmentItems += [ordered]@{
            codigo = 'EDU-DEMO-MAT-{0:D3}' -f $i
            solicitud = [string]$applications[$i - 1].recordID
            estudiante = [string]$prospects[$i - 1].recordID
            programa = [string]$programs[($i - 1) % 10].recordID
            periodo = '2026-2'
            valor = 3000000 + ($i * 180000)
            descuentoBeca = $(if ($i % 5 -eq 0) { 500000 } else { 0 })
            estadoPago = @('Pendiente', 'Parcial', 'Pagado', 'Financiado')[$i % 4]
            estado = $(if ($i -le 22) { 'Confirmada' } else { 'Preparada' })
            fechaMatricula = $now.AddDays(-($i % 20)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            observaciones = 'Matrícula sintética.'
            externalID = 'SIS-MAT-{0:D3}' -f $i
        }
    }
    $enrollments = Ensure-AberaRecords matriculas codigo $enrollmentItems

    return [ordered]@{
        prospectos = $prospects.Count
        programas = $programs.Count
        solicitudes = $applications.Count
        citas = $appointments.Count
        matriculas = $enrollments.Count
    }
}

function New-AberaServiceData {
    $now = (Get-Date).ToUniversalTime()
    $names = Get-AberaDemoNames
    $cities = Get-AberaDemoCities

    $customerItems = @()
    for ($i = 1; $i -le 100; $i++) {
        $customerItems += [ordered]@{
            tipo = $(if ($i -le 70) { 'Cliente' } else { 'Prospecto' })
            tipoPersona = $(if ($i % 4 -eq 0) { 'Empresa' } else { 'Persona' })
            nombre = $(if ($i % 4 -eq 0) { "Empresa Demo $i S.A.S." } else { $names[($i - 1) % $names.Count] })
            identificacion = '900{0:D6}' -f $i
            contacto = $names[($i + 3) % $names.Count]
            telefono = '+57 302{0:D7}' -f $i
            correo = 'serv.demo.{0:D3}@abera.local' -f $i
            autorizaTratamientoDatos = '1'
            departamento = @('Bogotá D.C.', 'Antioquia', 'Valle del Cauca', 'Atlántico')[$i % 4]
            ciudad = $cities[$i % $cities.Count]
            direccion = "Carrera $((($i * 3) % 90) + 1) # $((($i * 7) % 80) + 1)-20"
            segmento = @('Hogar', 'Pyme', 'Corporativo')[$i % 3]
            estadoComercial = @('Nuevo', 'Diagnóstico', 'Cotización', 'Cliente', 'Perdido')[$i % 5]
            responsable = $script:AberaSeed.ActorId
            proximoContacto = $now.AddDays(1 + ($i % 20)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            notas = 'Cliente sintético de servicios.'
            externalID = 'SERV-CLIENT-{0:D3}' -f $i
        }
    }
    $customers = Ensure-AberaRecords clientes-prospectos correo $customerItems

    $assetItems = @()
    for ($i = 1; $i -le 60; $i++) {
        $assetItems += [ordered]@{
            codigo = 'SERV-DEMO-ACT-{0:D3}' -f $i
            cliente = [string]$customers[($i - 1) % 70].recordID
            tipoEquipo = @('Aire acondicionado', 'Sistema eléctrico', 'Bomba', 'Equipo de refrigeración', 'UPS')[$i % 5]
            marca = @('Samsung', 'LG', 'Siemens', 'Schneider', 'Haceb')[$i % 5]
            modelo = "Modelo-$((($i - 1) % 12) + 1)"
            serial = 'SER-{0:D8}' -f $i
            fechaInstalacion = $now.AddDays(-200 - $i).ToString('yyyy-MM-dd')
            garantiaHasta = $now.AddDays(180 - $i).ToString('yyyy-MM-dd')
            estado = @('Operativo', 'Operativo', 'Requiere mantenimiento', 'Fuera de servicio')[$i % 4]
            proximoMantenimiento = $now.AddDays(($i % 90) - 20).ToString('yyyy-MM-dd')
            ubicacion = 'Sede principal'
            observaciones = 'Activo sintético.'
            externalID = 'IOT-ACT-{0:D3}' -f $i
        }
    }
    $assets = Ensure-AberaRecords activos codigo $assetItems

    $requestItems = @()
    for ($i = 1; $i -le 100; $i++) {
        $requestItems += [ordered]@{
            codigo = 'SERV-DEMO-SOL-{0:D3}' -f $i
            cliente = [string]$customers[($i - 1) % $customers.Count].recordID
            activo = [string]$assets[($i - 1) % $assets.Count].recordID
            canal = @('Teléfono', 'WhatsApp', 'Correo', 'Web')[$i % 4]
            tipoServicio = @('Instalación', 'Reparación', 'Mantenimiento', 'Inspección', 'Diagnóstico remoto')[$i % 5]
            prioridad = @('Baja', 'Media', 'Alta', 'Crítica')[$i % 4]
            descripcion = 'Solicitud técnica sintética para demostración.'
            estado = @('Recibida', 'En diagnóstico', 'Cotizada', 'Aceptada', 'Programada', 'Ejecutada', 'Cerrada', 'Perdida')[$i % 8]
            responsable = $script:AberaSeed.ActorId
            vencimientoSLA = $now.AddHours(2 + ($i % 48)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            slaIncumplido = [string]($(if ($i % 11 -eq 0) { 1 } else { 0 }))
            externalID = 'SERV-TICKET-{0:D3}' -f $i
            proveedorExterno = 'Portal demo'
            estadoIntegracion = 'Local'
        }
    }
    $requests = Ensure-AberaRecords solicitudes codigo $requestItems

    $quoteItems = @()
    for ($i = 1; $i -le 70; $i++) {
        $baseValue = 180000 + ($i * 65000)
        $quoteItems += [ordered]@{
            codigo = 'SERV-DEMO-COT-{0:D3}' -f $i
            solicitud = [string]$requests[$i - 1].recordID
            cliente = [string]$customers[($i - 1) % $customers.Count].recordID
            alcance = 'Diagnóstico, mano de obra y materiales descritos.'
            valorAntesImpuestos = $baseValue
            impuestos = [Math]::Round($baseValue * 0.19, 0)
            total = [Math]::Round($baseValue * 1.19, 0)
            fechaVencimiento = $now.AddDays(15 + ($i % 15)).ToString('yyyy-MM-dd')
            estado = @('Borrador', 'Enviada', 'Aceptada', 'Rechazada')[$i % 4]
            fechaAceptacion = $(if ($i % 4 -eq 2) { $now.AddDays(-($i % 10)).ToString('yyyy-MM-ddTHH:mm:ssZ') } else { $null })
            observaciones = 'Cotización sintética.'
            externalID = 'ERP-COT-{0:D3}' -f $i
        }
    }
    $quotes = Ensure-AberaRecords cotizaciones codigo $quoteItems

    $orderItems = @()
    for ($i = 1; $i -le 50; $i++) {
        $start = $now.AddDays(($i % 25) - 5).AddHours(8 + ($i % 8))
        $orderItems += [ordered]@{
            codigo = 'SERV-DEMO-OT-{0:D3}' -f $i
            solicitud = [string]$requests[$i - 1].recordID
            cotizacion = [string]$quotes[$i - 1].recordID
            cliente = [string]$customers[($i - 1) % $customers.Count].recordID
            activo = [string]$assets[($i - 1) % $assets.Count].recordID
            tecnico = $script:AberaSeed.ActorId
            direccion = [string](Get-AberaRecordValue $customers[($i - 1) % $customers.Count] direccion)
            inicio = $start.ToString('yyyy-MM-ddTHH:mm:ssZ')
            fin = $start.AddHours(2 + ($i % 3)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            estado = @('Programada', 'En camino', 'En ejecución', 'Completada')[$i % 4]
            diagnostico = 'Diagnóstico técnico sintético.'
            solucion = 'Trabajo documentado para demostración.'
            resolucionPrimeraVisita = [string]($(if ($i % 4 -ne 0) { 1 } else { 0 }))
            valorFinal = 220000 + ($i * 72000)
            proximoMantenimiento = $now.AddDays(120 + $i).ToString('yyyy-MM-dd')
            externalID = 'FSM-OT-{0:D3}' -f $i
        }
    }
    $orders = Ensure-AberaRecords ordenes-trabajo codigo $orderItems

    return [ordered]@{
        clientesProspectos = $customers.Count
        activos = $assets.Count
        solicitudes = $requests.Count
        cotizaciones = $quotes.Count
        ordenesTrabajo = $orders.Count
    }
}

function New-AberaContactCenterData {
    $now = (Get-Date).ToUniversalTime()
    $names = Get-AberaDemoNames
    $cities = Get-AberaDemoCities

    $contactItems = @()
    for ($i = 1; $i -le 200; $i++) {
        $name = $names[($i - 1) % $names.Count].Split(' ', 2)
        $contactItems += [ordered]@{
            nombres = $name[0]
            apellidos = $name[1]
            identificacion = '11{0:D8}' -f $i
            telefonoPrincipal = '+57 303{0:D7}' -f $i
            telefonoAlterno = '+57 304{0:D7}' -f $i
            correo = 'call.demo.{0:D3}@abera.local' -f $i
            ciudad = $cities[$i % $cities.Count]
            autorizaTratamientoDatos = '1'
            autorizaContacto = '1'
            noLlamar = '0'
            productoInteres = @('Seguro', 'Internet empresarial', 'Educación', 'Crédito', 'Software')[$i % 5]
            presupuesto = 500000 + (($i * 135000) % 12000000)
            estadoComercial = @('Cargado', 'Intentado', 'Contactado', 'Calificado', 'Oportunidad', 'Venta', 'No interesado')[$i % 7]
            responsable = $script:AberaSeed.ActorId
            totalIntentos = $i % 6
            proximaLlamada = $now.AddDays($i % 12).ToString('yyyy-MM-ddTHH:mm:ssZ')
            notas = 'Contacto sintético.'
            externalID = 'CALL-CONTACT-{0:D3}' -f $i
            estadoIntegracion = 'Local'
        }
    }
    $contacts = Ensure-AberaRecords contactos-leads correo $contactItems

    $campaignItems = @()
    for ($i = 1; $i -le 4; $i++) {
        $campaignItems += [ordered]@{
            codigo = 'CALL-DEMO-CAMP-{0:D2}' -f $i
            nombre = @('Venta cruzada', 'Prospectos digitales', 'Recuperación de clientes', 'Renovación comercial')[$i - 1]
            objetivo = @('Venta adicional', 'Venta', 'Recuperación', 'Venta')[$i - 1]
            producto = @('Plan Premium', 'Servicio empresarial', 'Plan de regreso', 'Renovación anual')[$i - 1]
            guion = 'Guion de demostración con saludo, descubrimiento y cierre.'
            prioridad = @('Alta', 'Media', 'Alta', 'Media')[$i - 1]
            fechaInicio = $now.AddDays(-10).ToString('yyyy-MM-dd')
            fechaFin = $now.AddDays(30).ToString('yyyy-MM-dd')
            horaInicio = '08:00'
            horaFin = '18:00'
            metaContactos = 120
            metaVentas = 35
            maximoIntentos = 5
            estado = 'Activa'
            externalID = 'DIALER-CAMP-{0:D2}' -f $i
        }
    }
    $campaigns = Ensure-AberaRecords campanas codigo $campaignItems

    $dialItems = @()
    for ($i = 1; $i -le 200; $i++) {
        $dialItems += [ordered]@{
            codigo = 'CALL-DEMO-REG-{0:D3}' -f $i
            campana = [string]$campaigns[($i - 1) % 4].recordID
            contacto = [string]$contacts[$i - 1].recordID
            agente = $script:AberaSeed.ActorId
            prioridad = @('Baja', 'Media', 'Alta')[$i % 3]
            intentos = $i % 5
            proximoIntento = $now.AddHours($i % 72).ToString('yyyy-MM-ddTHH:mm:ssZ')
            estado = @('Pendiente', 'En gestión', 'Callback', 'Contactado', 'Calificado', 'Cerrado')[$i % 6]
            ultimoResultado = @('Pendiente', 'No contesta', 'Callback', 'Contactado', 'Calificado', 'No interesado')[$i % 6]
            fechaAsignacion = $now.AddDays(-($i % 20)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            externalID = 'DIALER-REG-{0:D3}' -f $i
        }
    }
    $dialRecords = Ensure-AberaRecords registros-campana codigo $dialItems

    $callItems = @()
    for ($i = 1; $i -le 400; $i++) {
        $start = $now.AddMinutes(-($i * 17))
        $disposition = if ($i -le 40) { 'Calificado' } else {
            @('No contesta', 'Ocupado', 'Callback', 'Contactado', 'No interesado')[$i % 5]
        }
        $callItems += [ordered]@{
            registroCampana = [string]$dialRecords[($i - 1) % 200].recordID
            contacto = [string]$contacts[($i - 1) % 200].recordID
            campana = [string]$campaigns[($i - 1) % 4].recordID
            agente = $script:AberaSeed.ActorId
            direccion = 'Saliente'
            inicio = $start.ToString('yyyy-MM-ddTHH:mm:ssZ')
            fin = $start.AddSeconds(40 + ($i % 420)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            duracionSegundos = 40 + ($i % 420)
            disposicion = $disposition
            resumen = 'Llamada sintética de demostración.'
            callback = $now.AddDays(1 + ($i % 7)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            externalID = 'CALL-DEMO-{0:D4}' -f $i
            proveedorExterno = 'Telefonía demo'
            grabacionURL = "https://example.invalid/recordings/$i"
            transcripcionURL = "https://example.invalid/transcripts/$i"
            estadoIntegracion = 'Local'
        }
    }
    $calls = Ensure-AberaRecords llamadas externalID $callItems

    $opportunityItems = @()
    for ($i = 1; $i -le 40; $i++) {
        $opportunityItems += [ordered]@{
            codigo = 'CALL-OPP-{0}' -f $calls[$i - 1].recordID
            contacto = [string]$contacts[$i - 1].recordID
            campana = [string]$campaigns[($i - 1) % 4].recordID
            closer = $script:AberaSeed.ActorId
            producto = [string](Get-AberaRecordValue $campaigns[($i - 1) % 4] producto)
            etapa = @('Calificada', 'Contacto comercial', 'Propuesta', 'Negociación', 'Venta')[$i % 5]
            valor = 900000 + ($i * 280000)
            probabilidad = @(25, 40, 60, 80, 100)[$i % 5]
            fechaEstimadaCierre = $now.AddDays(5 + $i).ToString('yyyy-MM-dd')
            observaciones = 'Oportunidad sintética.'
        }
    }
    $opportunities = Ensure-AberaRecords oportunidades codigo $opportunityItems

    return [ordered]@{
        contactos = $contacts.Count
        campanas = $campaigns.Count
        registrosCampana = $dialRecords.Count
        llamadas = $calls.Count
        oportunidades = $opportunities.Count
    }
}

function New-AberaSupportData {
    $now = (Get-Date).ToUniversalTime()
    $names = Get-AberaDemoNames

    $customerItems = @()
    for ($i = 1; $i -le 60; $i++) {
        $customerItems += [ordered]@{
            codigo = 'SUP-DEMO-CLI-{0:D3}' -f $i
            tipo = $(if ($i % 3 -eq 0) { 'Empresa' } else { 'Persona' })
            nombre = $(if ($i % 3 -eq 0) { "Cliente Empresarial $i S.A.S." } else { $names[($i - 1) % $names.Count] })
            identificacion = '901{0:D6}' -f $i
            contactoPrincipal = $names[($i + 2) % $names.Count]
            telefono = '+57 305{0:D7}' -f $i
            correo = 'sup.demo.{0:D3}@abera.local' -f $i
            autorizaTratamientoDatos = '1'
            segmento = @('Emprendimiento', 'Pyme', 'Corporativo')[$i % 3]
            planActual = @('Básico', 'Estándar', 'Premium')[$i % 3]
            responsable = $script:AberaSeed.ActorId
            valorRecurrente = 350000 + ($i * 95000)
            salud = @('Saludable', 'Saludable', 'Atención', 'En riesgo')[$i % 4]
            satisfaccion = 2 + ($i % 4)
            riesgo = @('Bajo', 'Bajo', 'Medio', 'Alto')[$i % 4]
            notas = 'Cliente sintético de soporte.'
            externalID = 'CSM-CLIENT-{0:D3}' -f $i
        }
    }
    $customers = Ensure-AberaRecords clientes codigo $customerItems

    $contractItems = @()
    for ($i = 1; $i -le 80; $i++) {
        $start = $now.AddDays(-250 - ($i % 90))
        $contractItems += [ordered]@{
            codigo = 'SUP-DEMO-CON-{0:D3}' -f $i
            cliente = [string]$customers[($i - 1) % $customers.Count].recordID
            producto = @('Plataforma CRM', 'Mesa de ayuda', 'Analítica', 'Automatización')[$i % 4]
            nivelSoporte = @('Básico', 'Estándar', 'Premium')[$i % 3]
            fechaInicio = $start.ToString('yyyy-MM-dd')
            fechaFin = $start.AddDays(365).ToString('yyyy-MM-dd')
            renovacionAutomatica = [string]($i % 2)
            valor = 4800000 + ($i * 320000)
            slaRespuestaHoras = @(24, 8, 2)[$i % 3]
            slaResolucionHoras = @(72, 24, 8)[$i % 3]
            estado = 'Activo'
            observaciones = 'Contrato sintético.'
            externalID = 'BILL-CON-{0:D3}' -f $i
        }
    }
    $contracts = Ensure-AberaRecords contratos codigo $contractItems

    $caseItems = @()
    for ($i = 1; $i -le 150; $i++) {
        $opened = $now.AddHours(-($i % 120))
        $caseItems += [ordered]@{
            codigo = 'SUP-DEMO-CASO-{0:D3}' -f $i
            cliente = [string]$customers[($i - 1) % $customers.Count].recordID
            contrato = [string]$contracts[($i - 1) % $contracts.Count].recordID
            contacto = $names[($i + 3) % $names.Count]
            producto = @('Plataforma CRM', 'Mesa de ayuda', 'Analítica', 'Automatización')[$i % 4]
            canal = @('Correo', 'Teléfono', 'Chat', 'WhatsApp', 'Portal')[$i % 5]
            categoria = @('Acceso', 'Configuración', 'Error', 'Consulta', 'Integración')[$i % 5]
            asunto = "Caso de demostración #$i"
            descripcion = 'Descripción sintética de una solicitud de soporte.'
            prioridad = @('Baja', 'Media', 'Alta', 'Crítica')[$i % 4]
            estado = @('Nuevo', 'Asignado', 'En progreso', 'Esperando cliente', 'Resuelto', 'Cerrado')[$i % 6]
            agente = $script:AberaSeed.ActorId
            primeraRespuesta = $opened.AddHours(1 + ($i % 4)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            fechaResolucion = $(if ($i % 6 -ge 4) { $opened.AddHours(5 + ($i % 20)).ToString('yyyy-MM-ddTHH:mm:ssZ') } else { $null })
            vencimientoRespuesta = $opened.AddHours(8).ToString('yyyy-MM-ddTHH:mm:ssZ')
            vencimientoResolucion = $opened.AddHours(24).ToString('yyyy-MM-ddTHH:mm:ssZ')
            estadoSLA = @('En tiempo', 'En tiempo', 'En riesgo', 'Incumplido')[$i % 4]
            slaIncumplido = [string]($(if ($i % 4 -eq 3) { 1 } else { 0 }))
            resolucion = $(if ($i % 6 -ge 4) { 'Resolución sintética documentada.' } else { $null })
            resolucionPrimerContacto = [string]($(if ($i % 3 -ne 0) { 1 } else { 0 }))
            csat = 1 + ($i % 5)
            externalID = 'HELPDESK-{0:D3}' -f $i
            proveedorExterno = 'Canal demo'
            estadoIntegracion = 'Local'
        }
    }
    $cases = Ensure-AberaRecords casos codigo $caseItems

    $interactionItems = @()
    for ($i = 1; $i -le 250; $i++) {
        $interactionItems += [ordered]@{
            caso = [string]$cases[($i - 1) % $cases.Count].recordID
            cliente = [string]$customers[($i - 1) % $customers.Count].recordID
            canal = @('Correo', 'Teléfono', 'Chat', 'WhatsApp', 'Nota interna')[$i % 5]
            agente = $script:AberaSeed.ActorId
            fecha = $now.AddMinutes(-($i * 19)).ToString('yyyy-MM-ddTHH:mm:ssZ')
            tipo = @('Respuesta al cliente', 'Mensaje del cliente', 'Nota interna')[$i % 3]
            resumen = 'Interacción sintética de soporte.'
            externalID = 'SUP-DEMO-INT-{0:D4}' -f $i
            proveedorExterno = 'Canal demo'
        }
    }
    $interactions = Ensure-AberaRecords interacciones externalID $interactionItems

    $renewalItems = @()
    for ($i = 1; $i -le 40; $i++) {
        $contract = $contracts[$i - 1]
        $renewalItems += [ordered]@{
            codigo = 'REN-{0}' -f $contract.recordID
            cliente = [string](Get-AberaRecordValue $contract cliente)
            contrato = [string]$contract.recordID
            responsable = $script:AberaSeed.ActorId
            fechaGestion = $now.AddDays(($i % 30) - 15).ToString('yyyy-MM-dd')
            fechaVencimiento = [string](Get-AberaRecordValue $contract fechaFin)
            etapa = @('Próxima a vencer', 'Contactada', 'Propuesta', 'Comprometida', 'Renovada', 'Perdida')[$i % 6]
            valorActual = [string](Get-AberaRecordValue $contract valor)
            nuevoValor = 5200000 + ($i * 350000)
            probabilidad = @(60, 65, 75, 90, 100, 0)[$i % 6]
            riesgo = @('Bajo', 'Medio', 'Alto')[$i % 3]
            ventaAdicional = [string]($i % 2)
            valorExpansion = $(if ($i % 2 -eq 1) { 600000 + ($i * 50000) } else { 0 })
            motivoPerdida = $(if ($i % 6 -eq 5) { 'El cliente eligió otra solución.' } else { $null })
            observaciones = 'Renovación sintética.'
        }
    }
    $renewals = Ensure-AberaRecords renovaciones codigo $renewalItems

    return [ordered]@{
        clientes = $customers.Count
        contratos = $contracts.Count
        casos = $cases.Count
        interacciones = $interactions.Count
        renovaciones = $renewals.Count
    }
}

function Invoke-AberaDemoSeed {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [ValidateSet(
            'automotriz-co',
            'admisiones-educativas-co',
            'servicios-tecnicos-co',
            'centro-contacto-co',
            'soporte-renovaciones-co'
        )]
        [string]$TemplateId,
        [string]$BaseUrl = 'http://localhost:8080',
        [Parameter(Mandatory = $true)]
        [string]$ClientId,
        [string]$ClientSecret = $env:ABERA_CLIENT_SECRET,
        [string]$Email = $env:ABERA_ADMIN_EMAIL,
        [string]$Password = $env:ABERA_ADMIN_PASSWORD
    )

    $ErrorActionPreference = 'Stop'
    foreach ($secret in @(
        @{ Name = 'ABERA_CLIENT_SECRET'; Value = $ClientSecret },
        @{ Name = 'ABERA_ADMIN_EMAIL'; Value = $Email },
        @{ Name = 'ABERA_ADMIN_PASSWORD'; Value = $Password }
    )) {
        if ([string]::IsNullOrWhiteSpace([string]$secret.Value)) {
            throw "Defina $($secret.Name) o el parámetro correspondiente."
        }
    }

    $BaseUrl = $BaseUrl.TrimEnd('/')
    Write-Output "Autenticando y preparando '$TemplateId'..."
    $accessToken = Get-AberaAccessToken `
        -BaseUrl $BaseUrl `
        -ClientId $ClientId `
        -ClientSecret $ClientSecret `
        -Email $Email `
        -Password $Password

    $headers = @{
        Accept = 'application/json'
        Authorization = "Bearer $accessToken"
    }
    $composeApi = "$BaseUrl/api/compose"
    $namespaceResponse = Invoke-RestMethod `
        -Method Get `
        -Headers $headers `
        -Uri "$composeApi/namespace/?slug=$([uri]::EscapeDataString($TemplateId))&limit=20"
    $namespace = @($namespaceResponse.response.set) |
        Where-Object { $_.slug -eq $TemplateId } |
        Select-Object -First 1
    if (-not $namespace) {
        throw "No existe el espacio de trabajo '$TemplateId'."
    }

    $namespaceId = [string]$namespace.namespaceID
    $moduleResponse = Invoke-RestMethod `
        -Method Get `
        -Headers $headers `
        -Uri "$composeApi/namespace/$namespaceId/module/?limit=100"
    $moduleIds = @{}
    foreach ($module in @($moduleResponse.response.set)) {
        $moduleIds[[string]$module.handle] = [string]$module.moduleID
    }

    $userResponse = Invoke-RestMethod `
        -Method Get `
        -Headers $headers `
        -Uri "$BaseUrl/api/system/users/?email=$([uri]::EscapeDataString($Email))&limit=20"
    $actor = @($userResponse.response.set) |
        Where-Object { $_.email -eq $Email } |
        Select-Object -First 1
    if (-not $actor) {
        throw "No se encontró el usuario autenticado '$Email'."
    }

    $script:AberaSeed = [pscustomobject]@{
        BaseUrl = $BaseUrl
        ComposeApi = $composeApi
        Headers = $headers
        NamespaceId = $namespaceId
        ModuleIds = $moduleIds
        ActorId = [string]$actor.userID
    }

    $requiredModules = switch ($TemplateId) {
        'automotriz-co' { @('prospectos', 'vehiculos', 'pruebas-manejo', 'oportunidades', 'ordenes-servicio') }
        'admisiones-educativas-co' { @('prospectos', 'programas', 'solicitudes', 'citas-admision', 'matriculas') }
        'servicios-tecnicos-co' { @('clientes-prospectos', 'activos', 'solicitudes', 'cotizaciones', 'ordenes-trabajo') }
        'centro-contacto-co' { @('contactos-leads', 'campanas', 'registros-campana', 'llamadas', 'oportunidades') }
        'soporte-renovaciones-co' { @('clientes', 'contratos', 'casos', 'interacciones', 'renovaciones') }
    }
    foreach ($module in $requiredModules) {
        if (-not $moduleIds.ContainsKey($module)) {
            throw "No existe el módulo requerido '$module'."
        }
    }

    $summary = switch ($TemplateId) {
        'automotriz-co' { New-AberaAutomotiveData }
        'admisiones-educativas-co' { New-AberaEducationData }
        'servicios-tecnicos-co' { New-AberaServiceData }
        'centro-contacto-co' { New-AberaContactCenterData }
        'soporte-renovaciones-co' { New-AberaSupportData }
    }
    $summary['template'] = $TemplateId
    $summary['namespaceID'] = $namespaceId
    Write-Output ($summary | ConvertTo-Json -Depth 5)

    $accessToken = $null
    $headers.Authorization = $null
    $ClientSecret = $null
    $Password = $null
}
