# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

[CmdletBinding()]
param(
    [string]$BaseUrl = 'http://localhost:8080',
    [Parameter(Mandatory = $true)]
    [string]$ClientId,
    [string]$ClientSecret = $env:ABERA_CLIENT_SECRET,
    [string]$Email = 'admin@inmobiliaria.local',
    [string]$Password = $env:ABERA_ADMIN_PASSWORD,
    [ValidateRange(100, 1000)]
    [int]$LeadCount = 100,
    [ValidateRange(24, 200)]
    [int]$PropertyCount = 36,
    [ValidateRange(0, 500)]
    [int]$AppointmentCount = 60,
    [ValidateRange(0, 1000)]
    [int]$ActivityCount = 100,
    [ValidateRange(0, 500)]
    [int]$NegotiationCount = 35,
    [string]$NamespaceSlug = 'inmobiliaria-co'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($ClientSecret)) {
    throw 'Defina ABERA_CLIENT_SECRET o use -ClientSecret.'
}

if ([string]::IsNullOrWhiteSpace($Password)) {
    throw 'Defina ABERA_ADMIN_PASSWORD o use -Password.'
}

$BaseUrl = $BaseUrl.TrimEnd('/')
$redirectUri = "$BaseUrl/auth/callback"
$invariantCulture = [System.Globalization.CultureInfo]::InvariantCulture

function Invoke-WithoutRedirect {
    param(
        [Parameter(Mandatory = $true)]
        [Microsoft.PowerShell.Commands.WebRequestSession]$Session,
        [Parameter(Mandatory = $true)]
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
        StatusCode = [int]$response.StatusCode
        Location   = [string]$response.Headers['Location']
    }
}

function Get-QueryParameter {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Uri,
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    $query = ([uri]$Uri).Query.TrimStart('?')
    foreach ($item in $query.Split('&')) {
        $parts = $item.Split('=', 2)
        if ($parts.Count -eq 2 -and $parts[0] -eq $Name) {
            return [uri]::UnescapeDataString($parts[1])
        }
    }

    return $null
}

function Get-CortezaAccessToken {
    $session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    $loginPage = Invoke-WebRequest `
        -UseBasicParsing `
        -WebSession $session `
        -Uri "$BaseUrl/auth/login"

    $match = [regex]::Match(
        $loginPage.Content,
        'name="same-site-authenticity-token" value="([^"]+)"'
    )

    if (-not $match.Success) {
        throw 'No se encontró el token de autenticidad del formulario de acceso.'
    }

    $loginResponse = Invoke-WebRequest `
        -UseBasicParsing `
        -WebSession $session `
        -Method Post `
        -Uri "$BaseUrl/auth/login" `
        -Body @{
            'same-site-authenticity-token' = $match.Groups[1].Value
            email                          = $Email
            password                       = $Password
        }

    $authenticatedSession = @(
        $session.Cookies.GetCookies([uri]"$BaseUrl/auth/") |
            Where-Object { $_.Name -eq 'session' }
    )
    if ($authenticatedSession.Count -eq 0) {
        throw 'Corteza rechazó las credenciales del usuario.'
    }

    $state = 'abera-demo-' + [guid]::NewGuid().ToString('N')
    $authorizeUri = (
        "$BaseUrl/auth/oauth2/authorize" +
        "?client_id=$([uri]::EscapeDataString($ClientId))" +
        "&redirect_uri=$([uri]::EscapeDataString($redirectUri))" +
        '&response_mode=query' +
        '&response_type=code' +
        '&scope=profile%20api' +
        "&state=$state"
    )

    $currentUri = $authorizeUri
    $authorizationCode = $null

    for ($redirect = 0; $redirect -lt 8; $redirect++) {
        $step = Invoke-WithoutRedirect -Session $session -Uri $currentUri
        if ([string]::IsNullOrWhiteSpace($step.Location)) {
            throw "El flujo OAuth se detuvo sin redirección en $currentUri."
        }

        $nextUri = [uri]::new([uri]$BaseUrl, $step.Location).AbsoluteUri
        if ($nextUri.StartsWith($redirectUri, [System.StringComparison]::OrdinalIgnoreCase)) {
            $returnedState = Get-QueryParameter -Uri $nextUri -Name 'state'
            if ($returnedState -ne $state) {
                throw 'El estado devuelto por OAuth no coincide con el solicitado.'
            }

            $authorizationCode = Get-QueryParameter -Uri $nextUri -Name 'code'
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
            grant_type    = 'authorization_code'
            client_id     = $ClientId
            client_secret = $ClientSecret
            code          = $authorizationCode
            redirect_uri  = $redirectUri
        }

    if ([string]::IsNullOrWhiteSpace($tokenResponse.access_token)) {
        throw 'El servidor OAuth no devolvió un token de acceso.'
    }

    return [string]$tokenResponse.access_token
}

function ConvertTo-RecordValues {
    param(
        [Parameter(Mandatory = $true)]
        [System.Collections.IDictionary]$Values
    )

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
                $item.ToString($null, $invariantCulture)
            } else {
                [string]$item
            }

            $output += [pscustomobject]@{
                name  = [string]$name
                value = $text
            }
        }
    }

    return @($output)
}

function Get-RecordValues {
    param(
        [Parameter(Mandatory = $true)]
        $Record,
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    if ($Record.values -is [System.Collections.IDictionary]) {
        $value = $Record.values[$Name]
        if ($null -eq $value) {
            return @()
        }

        return @($value)
    }

    return @(
        $Record.values |
            Where-Object { $_.name -eq $Name } |
            ForEach-Object { $_.value }
    )
}

function Get-FirstRecordValue {
    param(
        [Parameter(Mandatory = $true)]
        $Record,
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    return @(Get-RecordValues -Record $Record -Name $Name) |
        Select-Object -First 1
}

Write-Output 'Autenticando con OAuth...'
$accessToken = Get-CortezaAccessToken
$apiHeaders = @{
    Accept        = 'application/json'
    Authorization = "Bearer $accessToken"
}
$composeApi = "$BaseUrl/api/compose"

function Invoke-CortezaGet {
    param([Parameter(Mandatory = $true)][string]$Path)

    return Invoke-RestMethod `
        -Method Get `
        -Headers $apiHeaders `
        -Uri ($composeApi + $Path)
}

function Get-CortezaRecords {
    param([Parameter(Mandatory = $true)][string]$ModuleId)

    $response = Invoke-CortezaGet `
        -Path "/namespace/$namespaceId/module/$ModuleId/record/?limit=500&incTotal=true"

    return @($response.response.set)
}

function New-CortezaRecord {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ModuleId,
        [Parameter(Mandatory = $true)]
        [System.Collections.IDictionary]$Values
    )

    $payload = @{
        values = @(ConvertTo-RecordValues -Values $Values)
    } | ConvertTo-Json -Depth 12 -Compress

    $response = Invoke-RestMethod `
        -Method Post `
        -Headers $apiHeaders `
        -ContentType 'application/json' `
        -Uri "$composeApi/namespace/$namespaceId/module/$ModuleId/record/" `
        -Body ([System.Text.Encoding]::UTF8.GetBytes($payload))

    $responseProperty = $response.PSObject.Properties['response']
    if (-not $responseProperty) {
        throw (
            'La API no devolvió el registro creado: ' +
            ($response | ConvertTo-Json -Depth 8 -Compress)
        )
    }

    return $responseProperty.Value
}

$namespaceResponse = Invoke-CortezaGet `
    -Path "/namespace/?slug=$([uri]::EscapeDataString($NamespaceSlug))&limit=20"
$namespace = @($namespaceResponse.response.set) |
    Where-Object { $_.slug -eq $NamespaceSlug } |
    Select-Object -First 1

if (-not $namespace) {
    throw "No existe el espacio de trabajo '$NamespaceSlug'."
}

$namespaceId = [string]$namespace.namespaceID
$moduleResponse = Invoke-CortezaGet -Path "/namespace/$namespaceId/module/?limit=100"
$moduleIds = @{}
foreach ($module in @($moduleResponse.response.set)) {
    $moduleIds[[string]$module.handle] = [string]$module.moduleID
}

foreach ($requiredModule in @(
    'inmuebles',
    'leads',
    'citas',
    'actividades',
    'negociaciones'
)) {
    if (-not $moduleIds.ContainsKey($requiredModule)) {
        throw "No existe el módulo requerido '$requiredModule'."
    }
}

$locations = @(
    [pscustomobject]@{ Department = 'Bogotá D.C.'; City = 'Bogotá'; Sector = 'Chapinero' },
    [pscustomobject]@{ Department = 'Antioquia'; City = 'Medellín'; Sector = 'El Poblado' },
    [pscustomobject]@{ Department = 'Valle del Cauca'; City = 'Cali'; Sector = 'Ciudad Jardín' },
    [pscustomobject]@{ Department = 'Atlántico'; City = 'Barranquilla'; Sector = 'Riomar' },
    [pscustomobject]@{ Department = 'Bolívar'; City = 'Cartagena'; Sector = 'Manga' },
    [pscustomobject]@{ Department = 'Santander'; City = 'Bucaramanga'; Sector = 'Cabecera' },
    [pscustomobject]@{ Department = 'Risaralda'; City = 'Pereira'; Sector = 'Pinares' },
    [pscustomobject]@{ Department = 'Quindío'; City = 'Armenia'; Sector = 'La Castellana' },
    [pscustomobject]@{ Department = 'Cundinamarca'; City = 'Chía'; Sector = 'La Balsa' },
    [pscustomobject]@{ Department = 'Boyacá'; City = 'Tunja'; Sector = 'Unicentro' },
    [pscustomobject]@{ Department = 'Tolima'; City = 'Ibagué'; Sector = 'El Vergel' },
    [pscustomobject]@{ Department = 'Meta'; City = 'Villavicencio'; Sector = 'Buque' }
)
$propertyTypes = @(
    'Apartamento',
    'Casa',
    'Oficina',
    'Local',
    'Finca',
    'Bodega',
    'Consultorio',
    'Lote'
)
$operations = @('Venta', 'Arriendo', 'Venta o arriendo')
$ownerFirstNames = @(
    'Camila',
    'Juan',
    'Marcela',
    'Andrés',
    'Paola',
    'Ricardo',
    'Diana',
    'Felipe'
)
$ownerLastNames = @(
    'Gómez',
    'Restrepo',
    'Martínez',
    'Rodríguez',
    'Herrera',
    'Ramírez',
    'Castro',
    'Moreno'
)

Write-Output 'Preparando inventario de inmuebles...'
$existingProperties = Get-CortezaRecords -ModuleId $moduleIds.inmuebles
$propertiesByCode = @{}
foreach ($record in $existingProperties) {
    $code = [string](Get-FirstRecordValue -Record $record -Name 'codigo')
    if ($code -like 'DEMO-*') {
        $propertiesByCode[$code] = $record
    }
}

$propertyEntries = @()
for ($index = 1; $index -le $PropertyCount; $index++) {
    $code = 'DEMO-{0:D3}' -f $index
    $location = $locations[($index - 1) % $locations.Count]
    $propertyType = $propertyTypes[($index - 1) % $propertyTypes.Count]
    $operation = $operations[($index - 1) % $operations.Count]
    $salePrice = 210000000 + (($index * 37000000) % 980000000)
    $rentPrice = 1450000 + (($index * 185000) % 5200000)
    $area = 48 + (($index * 11) % 230)
    $rooms = 1 + ($index % 5)
    $bathrooms = 1 + ($index % 4)
    $parking = $index % 3
    $status = if ($index -le ($PropertyCount - 6)) {
        'Disponible'
    } elseif ($index -le ($PropertyCount - 3)) {
        'Reservado'
    } elseif ($index -eq ($PropertyCount - 2)) {
        'Arrendado'
    } elseif ($index -eq ($PropertyCount - 1)) {
        'Vendido'
    } else {
        'Inactivo'
    }

    if ($propertiesByCode.ContainsKey($code)) {
        $record = $propertiesByCode[$code]
    } else {
        $values = [ordered]@{
            codigo                 = $code
            nombre                 = "$propertyType en $($location.Sector), $($location.City)"
            descripcion            = (
                "Inmueble de demostración con excelente ubicación en " +
                "$($location.Sector). Cuenta con espacios iluminados, " +
                'buen acceso vial y servicios cercanos.'
            )
            tipoOperacion          = $operation
            tipoInmueble           = $propertyType
            estado                 = $status
            departamento           = $location.Department
            ciudad                 = $location.City
            direccion              = "Carrera $((($index * 3) % 90) + 1) # $((($index * 7) % 80) + 1)-$('{0:D2}' -f (($index * 13) % 99))"
            administracion         = 180000 + (($index * 35000) % 650000)
            area                   = $area
            habitaciones           = $rooms
            banos                  = $bathrooms
            parqueaderos           = $parking
            estrato                = [string](2 + ($index % 5))
            propietarioNombre      = (
                $ownerFirstNames[($index - 1) % $ownerFirstNames.Count] +
                ' ' +
                $ownerLastNames[($index - 1) % $ownerLastNames.Count]
            )
            propietarioTelefono    = '+57 31{0:D8}' -f (10000000 + $index)
        }

        if ($operation -ne 'Arriendo') {
            $values['precioVenta'] = $salePrice
        }
        if ($operation -ne 'Venta') {
            $values['canonArrendamiento'] = $rentPrice
        }

        $record = New-CortezaRecord `
            -ModuleId $moduleIds.inmuebles `
            -Values $values
        $propertiesByCode[$code] = $record
    }

    $propertyEntries += [pscustomobject]@{
        Index     = $index
        Code      = $code
        Record    = $record
        RecordId  = [string]$record.recordID
        Department = [string](Get-FirstRecordValue -Record $record -Name 'departamento')
        City      = [string](Get-FirstRecordValue -Record $record -Name 'ciudad')
        Operation = [string](Get-FirstRecordValue -Record $record -Name 'tipoOperacion')
        SalePrice = [string](Get-FirstRecordValue -Record $record -Name 'precioVenta')
        RentPrice = [string](Get-FirstRecordValue -Record $record -Name 'canonArrendamiento')
    }

    if ($index % 10 -eq 0 -or $index -eq $PropertyCount) {
        Write-Output ("  Inmuebles listos: {0}/{1}" -f $index, $PropertyCount)
    }
}

$leadFirstNames = @(
    'Sofía',
    'Mateo',
    'Valentina',
    'Santiago',
    'Mariana',
    'Sebastián',
    'Isabella',
    'Nicolás',
    'Gabriela',
    'Samuel',
    'Laura',
    'Daniel',
    'Natalia',
    'Alejandro',
    'Carolina',
    'David',
    'Juliana',
    'Tomás',
    'Paula',
    'Julián'
)
$leadLastNames = @(
    'García',
    'Rodríguez',
    'Martínez',
    'López',
    'González',
    'Hernández',
    'Pérez',
    'Sánchez',
    'Ramírez',
    'Torres',
    'Flores',
    'Rivera',
    'Gómez',
    'Díaz',
    'Reyes',
    'Morales',
    'Ortiz',
    'Gutiérrez',
    'Ruiz',
    'Mendoza'
)
$leadStates = @()
$leadStates += @('Nuevo') * 18
$leadStates += @('En contacto') * 18
$leadStates += @('Calificado') * 18
$leadStates += @('Cita agendada') * 16
$leadStates += @('Negociación') * 12
$leadStates += @('Ganado') * 8
$leadStates += @('Perdido') * 6
$leadStates += @('Descartado') * 4

Write-Output 'Preparando leads y relaciones con inmuebles...'
$existingLeads = Get-CortezaRecords -ModuleId $moduleIds.leads
$leadsByEmail = @{}
foreach ($record in $existingLeads) {
    $leadEmail = [string](Get-FirstRecordValue -Record $record -Name 'correo')
    if ($leadEmail -like 'demo.crm.*@abera.local') {
        $leadsByEmail[$leadEmail] = $record
    }
}

$leadEntries = @()
for ($index = 1; $index -le $LeadCount; $index++) {
    $leadEmail = 'demo.crm.{0:D3}@abera.local' -f $index
    $location = $locations[($index - 1) % $locations.Count]
    $operation = $operations[($index - 1) % $operations.Count]
    $wantedTypes = @(
        $propertyTypes[($index - 1) % $propertyTypes.Count],
        $propertyTypes[($index + 2) % $propertyTypes.Count]
    ) | Select-Object -Unique

    $candidateProperties = @(
        $propertyEntries | Where-Object {
            $_.City -eq $location.City -and (
                $_.Operation -eq $operation -or
                $_.Operation -eq 'Venta o arriendo' -or
                $operation -eq 'Venta o arriendo'
            )
        }
    )
    if ($candidateProperties.Count -eq 0) {
        $candidateProperties = @(
            $propertyEntries | Where-Object { $_.City -eq $location.City }
        )
    }
    if ($candidateProperties.Count -eq 0) {
        $candidateProperties = @($propertyEntries)
    }

    $interestCount = 1 + (($index - 1) % 3)
    $interestProperties = @()
    for ($interestIndex = 0; $interestIndex -lt $interestCount; $interestIndex++) {
        $candidate = $candidateProperties[
            ($index + $interestIndex - 1) % $candidateProperties.Count
        ]
        $selectedPropertyIds = @(
            $interestProperties | ForEach-Object { $_.RecordId }
        )
        if ($candidate.RecordId -notin $selectedPropertyIds) {
            $interestProperties += $candidate
        }
    }

    if ($leadsByEmail.ContainsKey($leadEmail)) {
        $record = $leadsByEmail[$leadEmail]
    } else {
        if ($operation -eq 'Arriendo') {
            $minimumBudget = 1300000 + (($index * 90000) % 2200000)
            $maximumBudget = $minimumBudget + 1700000 + (($index % 4) * 450000)
        } else {
            $minimumBudget = 180000000 + (($index * 11000000) % 420000000)
            $maximumBudget = $minimumBudget + 180000000 + (($index % 5) * 70000000)
        }

        $nextContact = (Get-Date).ToUniversalTime().AddDays(
            1 + ($index % 21)
        ).AddHours(8 + ($index % 9))

        $values = [ordered]@{
            nombres                    = $leadFirstNames[($index - 1) % $leadFirstNames.Count]
            apellidos                  = $leadLastNames[(($index * 3) - 1) % $leadLastNames.Count]
            telefono                   = '+57 30{0:D8}' -f (20000000 + $index)
            correo                     = $leadEmail
            autorizaTratamientoDatos   = '1'
            tipoOperacion              = $operation
            tiposInmueble              = $wantedTypes
            departamentoBusqueda       = $location.Department
            ciudadBusqueda             = $location.City
            presupuestoMinimo          = $minimumBudget
            presupuestoMaximo          = $maximumBudget
            habitacionesMinimas        = 1 + ($index % 4)
            banosMinimos               = 1 + ($index % 3)
            inmueblesInteres            = @($interestProperties.RecordId)
            estado                     = $leadStates[($index - 1) % $leadStates.Count]
            proximoContacto             = $nextContact.ToString(
                'yyyy-MM-ddTHH:mm:ssZ',
                $invariantCulture
            )
            notas                       = (
                "Lead de demostración #$('{0:D3}' -f $index). " +
                "Busca $operation en $($location.City), " +
                "con interés en $($interestProperties.Count) inmueble(s)."
            )
        }

        $record = New-CortezaRecord `
            -ModuleId $moduleIds.leads `
            -Values $values
        $leadsByEmail[$leadEmail] = $record
    }

    $leadEntries += [pscustomobject]@{
        Index       = $index
        Email       = $leadEmail
        Record      = $record
        RecordId    = [string]$record.recordID
        AdvisorId   = [string](Get-FirstRecordValue -Record $record -Name 'responsable')
        InterestIds = @(Get-RecordValues -Record $record -Name 'inmueblesInteres')
        Operation   = [string](Get-FirstRecordValue -Record $record -Name 'tipoOperacion')
    }

    if ($index % 10 -eq 0 -or $index -eq $LeadCount) {
        Write-Output ("  Leads listos: {0}/{1}" -f $index, $LeadCount)
    }
}

$existingAppointments = Get-CortezaRecords -ModuleId $moduleIds.citas
$appointmentsByLead = @{}
foreach ($record in $existingAppointments) {
    $relatedLead = [string](Get-FirstRecordValue -Record $record -Name 'lead')
    if (-not [string]::IsNullOrWhiteSpace($relatedLead)) {
        $appointmentsByLead[$relatedLead] = $record
    }
}

$appointmentTarget = [Math]::Min($AppointmentCount, $leadEntries.Count)
Write-Output "Preparando $appointmentTarget citas relacionadas..."
for ($offset = 0; $offset -lt $appointmentTarget; $offset++) {
    $lead = $leadEntries[$offset]
    if (
        $appointmentsByLead.ContainsKey($lead.RecordId) -or
        [string]::IsNullOrWhiteSpace($lead.AdvisorId)
    ) {
        continue
    }

    $interestIds = @($lead.InterestIds)
    if ($interestIds.Count -eq 0) {
        $interestIds = @($propertyEntries[$offset % $propertyEntries.Count].RecordId)
    }

    $start = (Get-Date).ToUniversalTime().AddDays(
        2 + ($offset % 30)
    ).Date.AddHours(9 + ($offset % 8))
    $appointmentState = if ($offset % 11 -eq 0) {
        'Reprogramada'
    } elseif ($offset % 5 -eq 0) {
        'Confirmada'
    } else {
        'Pendiente'
    }
    $appointmentType = @(
        'Visita',
        'Visita',
        'Visita',
        'Llamada',
        'Virtual',
        'Oficina'
    )[$offset % 6]

    $record = New-CortezaRecord `
        -ModuleId $moduleIds.citas `
        -Values ([ordered]@{
            asunto          = "Visita de inmuebles - lead $($lead.Index)"
            lead            = $lead.RecordId
            asesor          = $lead.AdvisorId
            inmuebles       = $interestIds
            inicio          = $start.ToString('yyyy-MM-ddTHH:mm:ssZ', $invariantCulture)
            fin             = $start.AddHours(1).ToString('yyyy-MM-ddTHH:mm:ssZ', $invariantCulture)
            tipo            = $appointmentType
            estado          = $appointmentState
            puntoEncuentro  = 'Recepción o portería del primer inmueble'
            resultado       = 'Pendiente de realización'
            observaciones   = 'Cita creada por el sembrador API de la plantilla inmobiliaria.'
        })
    $appointmentsByLead[$lead.RecordId] = $record
}

$existingActivities = Get-CortezaRecords -ModuleId $moduleIds.actividades
$activitiesByLead = @{}
foreach ($record in $existingActivities) {
    $relatedLead = [string](Get-FirstRecordValue -Record $record -Name 'lead')
    if (-not [string]::IsNullOrWhiteSpace($relatedLead)) {
        $activitiesByLead[$relatedLead] = $record
    }
}

$activityTarget = [Math]::Min($ActivityCount, $leadEntries.Count)
Write-Output "Preparando $activityTarget actividades relacionadas..."
for ($offset = 0; $offset -lt $activityTarget; $offset++) {
    $lead = $leadEntries[$offset]
    if (
        $activitiesByLead.ContainsKey($lead.RecordId) -or
        [string]::IsNullOrWhiteSpace($lead.AdvisorId)
    ) {
        continue
    }

    $interestIds = @($lead.InterestIds)
    $activityDate = (Get-Date).ToUniversalTime().AddDays(-($offset % 15)).AddHours(-($offset % 8))
    $nextAction = (Get-Date).ToUniversalTime().AddDays(1 + ($offset % 14))
    $activityType = @(
        'Llamada',
        'WhatsApp',
        'Correo',
        'Seguimiento',
        'Nota'
    )[$offset % 5]

    $values = [ordered]@{
        lead          = $lead.RecordId
        asesor        = $lead.AdvisorId
        tipo          = $activityType
        fecha         = $activityDate.ToString('yyyy-MM-ddTHH:mm:ssZ', $invariantCulture)
        resultado     = 'Contacto realizado; el lead mantiene interés.'
        proximaAccion = $nextAction.ToString('yyyy-MM-ddTHH:mm:ssZ', $invariantCulture)
        observaciones = 'Seguimiento comercial de demostración creado mediante API.'
    }
    if ($interestIds.Count -gt 0) {
        $values['inmueble'] = $interestIds[0]
    }

    $record = New-CortezaRecord `
        -ModuleId $moduleIds.actividades `
        -Values $values
    $activitiesByLead[$lead.RecordId] = $record
}

$existingNegotiations = Get-CortezaRecords -ModuleId $moduleIds.negociaciones
$negotiationsByLead = @{}
foreach ($record in $existingNegotiations) {
    $relatedLead = [string](Get-FirstRecordValue -Record $record -Name 'lead')
    if (-not [string]::IsNullOrWhiteSpace($relatedLead)) {
        $negotiationsByLead[$relatedLead] = $record
    }
}

$negotiationTarget = [Math]::Min($NegotiationCount, $leadEntries.Count)
Write-Output "Preparando $negotiationTarget negociaciones relacionadas..."
for ($offset = 0; $offset -lt $negotiationTarget; $offset++) {
    $lead = $leadEntries[$offset]
    $interestIds = @($lead.InterestIds)
    if (
        $negotiationsByLead.ContainsKey($lead.RecordId) -or
        [string]::IsNullOrWhiteSpace($lead.AdvisorId) -or
        $interestIds.Count -eq 0
    ) {
        continue
    }

    $property = $propertyEntries |
        Where-Object { $_.RecordId -eq [string]$interestIds[0] } |
        Select-Object -First 1
    if (-not $property) {
        continue
    }

    $dealOperation = if (
        $lead.Operation -eq 'Arriendo' -or
        ($lead.Operation -eq 'Venta o arriendo' -and $offset % 2 -eq 1)
    ) {
        'Arriendo'
    } else {
        'Venta'
    }
    $publishedValue = if ($dealOperation -eq 'Arriendo') {
        if ($property.RentPrice) { [decimal]$property.RentPrice } else { 2800000 }
    } else {
        if ($property.SalePrice) { [decimal]$property.SalePrice } else { 420000000 }
    }
    $stage = @('Calificación', 'Oferta', 'Documentación', 'Cierre')[$offset % 4]
    $probability = @(25, 50, 75, 90)[$offset % 4]
    $dealState = if ($offset -ge ($negotiationTarget - 2)) {
        'Perdida'
    } elseif ($offset -ge ($negotiationTarget - 5)) {
        'Ganada'
    } else {
        'Abierta'
    }
    $closeDate = (Get-Date).ToUniversalTime().AddDays(20 + ($offset % 70))

    $values = [ordered]@{
        lead                 = $lead.RecordId
        inmueble             = $property.RecordId
        asesor               = $lead.AdvisorId
        tipoOperacion        = $dealOperation
        etapa                = $stage
        valorPublicado       = $publishedValue
        valorOfrecido        = [Math]::Round($publishedValue * (0.91 + (($offset % 5) / 100)), 0)
        probabilidad         = $probability
        fechaEstimadaCierre  = $closeDate.ToString('yyyy-MM-dd', $invariantCulture)
        estado               = $dealState
        observaciones        = 'Negociación de demostración vinculada al lead y al inmueble.'
    }
    if ($dealState -eq 'Perdida') {
        $values['motivoPerdida'] = 'El presupuesto final no coincidió con la oferta.'
    }

    $record = New-CortezaRecord `
        -ModuleId $moduleIds.negociaciones `
        -Values $values
    $negotiationsByLead[$lead.RecordId] = $record
}

Write-Output 'Validando datos creados mediante API...'
$finalProperties = Get-CortezaRecords -ModuleId $moduleIds.inmuebles
$finalLeads = Get-CortezaRecords -ModuleId $moduleIds.leads
$finalAppointments = Get-CortezaRecords -ModuleId $moduleIds.citas
$finalActivities = Get-CortezaRecords -ModuleId $moduleIds.actividades
$finalNegotiations = Get-CortezaRecords -ModuleId $moduleIds.negociaciones

$demoProperties = @(
    $finalProperties | Where-Object {
        [string](Get-FirstRecordValue -Record $_ -Name 'codigo') -like 'DEMO-*'
    }
)
$demoLeads = @(
    $finalLeads | Where-Object {
        [string](Get-FirstRecordValue -Record $_ -Name 'correo') -like 'demo.crm.*@abera.local'
    }
)
$leadsWithBudget = @(
    $demoLeads | Where-Object {
        -not [string]::IsNullOrWhiteSpace(
            [string](Get-FirstRecordValue -Record $_ -Name 'presupuestoMaximo')
        )
    }
)
$leadsWithProperties = @(
    $demoLeads | Where-Object {
        @(Get-RecordValues -Record $_ -Name 'inmueblesInteres').Count -gt 0
    }
)
$leadsWithAdvisor = @(
    $demoLeads | Where-Object {
        -not [string]::IsNullOrWhiteSpace(
            [string](Get-FirstRecordValue -Record $_ -Name 'responsable')
        )
    }
)
$appointmentsWithMultipleProperties = @(
    $finalAppointments | Where-Object {
        @(Get-RecordValues -Record $_ -Name 'inmuebles').Count -gt 1
    }
)

if ($demoLeads.Count -lt $LeadCount) {
    throw "Solo se encontraron $($demoLeads.Count) leads demo; se esperaban $LeadCount."
}
if ($leadsWithBudget.Count -lt $LeadCount) {
    throw 'No todos los leads demo tienen presupuesto.'
}
if ($leadsWithProperties.Count -lt $LeadCount) {
    throw 'No todos los leads demo tienen inmuebles de interés.'
}

$summary = [ordered]@{
    namespace                         = $NamespaceSlug
    inmueblesDemo                     = $demoProperties.Count
    leadsDemo                         = $demoLeads.Count
    leadsConPresupuesto               = $leadsWithBudget.Count
    leadsConInmuebles                 = $leadsWithProperties.Count
    leadsConAsesor                    = $leadsWithAdvisor.Count
    citasTotales                      = $finalAppointments.Count
    citasConVariosInmuebles           = $appointmentsWithMultipleProperties.Count
    actividadesTotales                = $finalActivities.Count
    negociacionesTotales              = $finalNegotiations.Count
}

Write-Output ($summary | ConvertTo-Json -Depth 4)

$accessToken = $null
$apiHeaders.Authorization = $null
$ClientSecret = $null
$Password = $null
