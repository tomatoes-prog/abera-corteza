# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

param(
    [string]$BaseUrl = 'http://localhost:8080',
    [Parameter(Mandatory = $true)][string]$PrivateKeyFile,
    [Parameter(Mandatory = $true)][string]$EnvelopeFile
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ExpectedModules = [ordered]@{
    'inmobiliaria-co' = 5
    'automotriz-co' = 7
    'admisiones-educativas-co' = 7
    'servicios-tecnicos-co' = 7
    'centro-contacto-co' = 5
    'soporte-renovaciones-co' = 6
}

function ConvertFrom-AberaBase64Url {
    param([Parameter(Mandatory = $true)][string]$Value)

    $normalized = $Value.Replace('-', '+').Replace('_', '/')
    switch ($normalized.Length % 4) {
        2 { $normalized += '==' }
        3 { $normalized += '=' }
        1 { throw 'Valor Base64 URL inválido.' }
    }
    return [Convert]::FromBase64String($normalized)
}

function Get-AberaBootstrapPayload {
    param([string]$KeyPath, [string]$EncryptedPath)

    $envelope = Get-Content -Raw -LiteralPath $EncryptedPath | ConvertFrom-Json
    if ($envelope.version -ne 1 -or $envelope.algorithm -ne 'RSA-OAEP-256+A256GCM') {
        throw 'El sobre de bootstrap no usa la versión y algoritmo esperados.'
    }

    $wrappedKey = ConvertFrom-AberaBase64Url $envelope.encryptedKey
    $nonce = ConvertFrom-AberaBase64Url $envelope.nonce
    $sealed = ConvertFrom-AberaBase64Url $envelope.ciphertext
    if ($sealed.Length -le 16) { throw 'El ciphertext del bootstrap es demasiado corto.' }

    $rsa = [Security.Cryptography.RSA]::Create()
    $aes = $null
    $plaintext = $null
    $key = $null
    try {
        $rsa.ImportFromPem([IO.File]::ReadAllText($KeyPath))
        $key = $rsa.Decrypt($wrappedKey, [Security.Cryptography.RSAEncryptionPadding]::OaepSHA256)
        $ciphertext = [byte[]]::new($sealed.Length - 16)
        $tag = [byte[]]::new(16)
        [Array]::Copy($sealed, 0, $ciphertext, 0, $ciphertext.Length)
        [Array]::Copy($sealed, $ciphertext.Length, $tag, 0, $tag.Length)
        $plaintext = [byte[]]::new($ciphertext.Length)
        $aad = [Text.Encoding]::ASCII.GetBytes('abera-bootstrap-v1')
        $aes = [Security.Cryptography.AesGcm]::new($key, 16)
        $aes.Decrypt($nonce, $ciphertext, $tag, $plaintext, $aad)
        return [Text.Encoding]::UTF8.GetString($plaintext) | ConvertFrom-Json
    } finally {
        if ($null -ne $aes) { $aes.Dispose() }
        $rsa.Dispose()
        foreach ($buffer in @($wrappedKey, $nonce, $sealed, $key, $plaintext)) {
            if ($null -ne $buffer) { [Array]::Clear($buffer, 0, $buffer.Length) }
        }
    }
}

function Invoke-AberaApi {
    param(
        [ValidateSet('Get', 'Post', 'Put', 'Delete')][string]$Method = 'Get',
        [Parameter(Mandatory = $true)][string]$Path,
        $Body
    )

    $parameters = @{
        Method = $Method
        Uri = "$BaseUrl$Path"
        Headers = $script:Headers
    }
    if ($PSBoundParameters.ContainsKey('Body')) {
        $parameters.ContentType = 'application/json'
        $parameters.Body = [Text.Encoding]::UTF8.GetBytes(($Body | ConvertTo-Json -Depth 30 -Compress))
    }
    $result = Invoke-RestMethod @parameters
    if ($result.PSObject.Properties['response']) {
        return $result.response
    }
    if ($result.PSObject.Properties['success']) {
        return $result.success
    }
    if (-not $result.PSObject.Properties['response']) {
        $properties = @($result.PSObject.Properties.Name) -join ', '
        $errorDetail = if ($result.PSObject.Properties['error']) {
            $result.error | ConvertTo-Json -Depth 10 -Compress
        } else {
            ''
        }
        throw "La API no devolvió response para $Method $Path. Propiedades: $properties. Error: $errorDetail"
    }
}

function Assert-Abera {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

$bootstrap = Get-AberaBootstrapPayload -KeyPath $PrivateKeyFile -EncryptedPath $EnvelopeFile
$token = $null
try {
    $tokenResponse = Invoke-RestMethod -Method Post -Uri "$BaseUrl/auth/oauth2/token" `
        -ContentType 'application/x-www-form-urlencoded' -Body @{
            grant_type = 'client_credentials'
            scope = 'api'
            client_id = [string]$bootstrap.oauth.clientID
            client_secret = [string]$bootstrap.oauth.clientSecret
        }
    $token = [string]$tokenResponse.access_token
    Assert-Abera (-not [string]::IsNullOrWhiteSpace($token)) 'OAuth no devolvió access_token.'
    $script:Headers = @{ Authorization = "Bearer $token" }

    $users = @((Invoke-AberaApi -Path '/api/system/users/?limit=1').set)
    Assert-Abera ($users.Count -eq 1) 'El modo admin no pudo consultar usuarios.'

    $namespaceResponse = Invoke-AberaApi -Path '/api/compose/namespace/?limit=100'
    $namespaces = @($namespaceResponse.set | Where-Object { $ExpectedModules.Contains($_.slug) })
    Assert-Abera ($namespaces.Count -eq 6) "La API devolvió $($namespaces.Count) espacios demo; se esperaban 6."

    $moduleTotal = 0
    $recordTotal = 0
    foreach ($namespace in $namespaces) {
        $slug = [string]$namespace.slug
        $modules = @((Invoke-AberaApi -Path "/api/compose/namespace/$($namespace.namespaceID)/module/?limit=100").set)
        Assert-Abera ($modules.Count -eq $ExpectedModules[$slug]) "${slug}: conteo de módulos inesperado."
        $moduleTotal += $modules.Count

        foreach ($module in $modules) {
            $moduleDescription = if ($module.meta.PSObject.Properties['description']) { [string]$module.meta.description } else { '' }
            Assert-Abera (-not [string]::IsNullOrWhiteSpace($moduleDescription)) `
                "$slug/$($module.handle): falta meta.description en la API."
            foreach ($field in @($module.fields)) {
                $fieldDescription = if ($field.options.PSObject.Properties['description']) { $field.options.description } else { $null }
                $viewDescription = if ($null -ne $fieldDescription -and $fieldDescription.PSObject.Properties['view']) { [string]$fieldDescription.view } else { '' }
                $editDescription = if ($null -ne $fieldDescription -and $fieldDescription.PSObject.Properties['edit']) { [string]$fieldDescription.edit } else { '' }
                Assert-Abera (-not [string]::IsNullOrWhiteSpace($viewDescription)) `
                    "$slug/$($module.handle).$($field.name): falta description.view."
                Assert-Abera (-not [string]::IsNullOrWhiteSpace($editDescription)) `
                    "$slug/$($module.handle).$($field.name): falta description.edit."
            }

            $records = @((Invoke-AberaApi -Path "/api/compose/namespace/$($namespace.namespaceID)/module/$($module.moduleID)/record/?limit=100").set)
            $expected = if ($slug -eq 'soporte-renovaciones-co' -and $module.handle -eq 'problemas-conocidos') { 20 } else { 100 }
            Assert-Abera ($records.Count -eq $expected) "$slug/$($module.handle): $($records.Count) registros; se esperaban $expected."
            $recordTotal += $records.Count
        }

        $pages = @((Invoke-AberaApi -Path "/api/compose/namespace/$($namespace.namespaceID)/page/?limit=100").set)
        $charts = @((Invoke-AberaApi -Path "/api/compose/namespace/$($namespace.namespaceID)/chart/?limit=100").set)
        Assert-Abera ($pages.Count -ge $ExpectedModules[$slug]) "${slug}: páginas insuficientes."
        Assert-Abera ($charts.Count -ge 3) "${slug}: gráficos insuficientes."
    }

    # Include disabled workflows explicitly; this endpoint's exclusive state
    # filter is not consistent across Corteza releases.
    $workflows = @((Invoke-AberaApi -Path '/api/automation/workflows/?limit=200&disabled=1&subWorkflow=1').set)
    $examples = @($workflows | Where-Object { $_.handle -like 'ejemplo-*' })
    if ($examples.Count -ne 12) {
        $returnedHandles = @($workflows | ForEach-Object { [string]$_.handle }) -join ', '
        Write-Output "Workflows desactivados devueltos: $($workflows.Count). Handles: $returnedHandles"
    }
    Assert-Abera ($examples.Count -eq 12) "Se esperaban 12 ejemplos externos; llegaron $($examples.Count)."
    Assert-Abera (@($examples | Where-Object enabled).Count -eq 0) 'Hay ejemplos externos habilitados.'
    Assert-Abera (@($workflows | Where-Object { [string]::IsNullOrWhiteSpace([string]$_.meta.description) }).Count -eq 0) `
        'Hay workflows sin descripción funcional.'

    $reports = @((Invoke-AberaApi -Path '/api/system/reports/?limit=100').set | Where-Object { $_.handle -like '*-co-*' })
    Assert-Abera ($reports.Count -eq 12) "La API devolvió $($reports.Count) reportes de plantilla; se esperaban 12."

    $support = $namespaces | Where-Object slug -eq 'soporte-renovaciones-co'
    $problemModule = @((Invoke-AberaApi -Path "/api/compose/namespace/$($support.namespaceID)/module/?limit=100").set) |
        Where-Object handle -eq 'problemas-conocidos'
    $smokeCode = "API-SMOKE-$([Guid]::NewGuid().ToString('N').Substring(0, 12))"
    $createdRecord = $null
    $createdWorkflow = $null
    try {
        $createdRecord = Invoke-AberaApi -Method Post `
            -Path "/api/compose/namespace/$($support.namespaceID)/module/$($problemModule.moduleID)/record/" `
            -Body @{ values = @(
                @{ name = 'codigo'; value = $smokeCode },
                @{ name = 'titulo'; value = 'Validación API temporal' },
                @{ name = 'estado'; value = 'En investigación' }
            ) }
        $recordUpdateBody = @{ values = @(
            @{ name = 'codigo'; value = $smokeCode },
            @{ name = 'titulo'; value = 'Validación API actualizada' },
            @{ name = 'estado'; value = 'Cerrado' }
        ) }
        if ($createdRecord.PSObject.Properties['updatedAt']) {
            $recordUpdateBody.updatedAt = $createdRecord.updatedAt
        }
        $updatedRecord = Invoke-AberaApi -Method Post `
            -Path "/api/compose/namespace/$($support.namespaceID)/module/$($problemModule.moduleID)/record/$($createdRecord.recordID)" `
            -Body $recordUpdateBody
        Assert-Abera ($updatedRecord.recordID -eq $createdRecord.recordID) 'La actualización de registro cambió su ID.'

        $workflowHandle = "api-smoke-$([Guid]::NewGuid().ToString('N').Substring(0, 12))"
        $createdWorkflow = Invoke-AberaApi -Method Post -Path '/api/automation/workflows/' -Body @{
            handle = $workflowHandle
            enabled = $false
            trace = $false
            keepSessions = 0
            meta = @{ name = 'Validación API temporal'; description = 'Workflow temporal creado por el smoke test.' }
            scope = @{}
            steps = @()
            paths = @()
            runAs = '0'
            ownedBy = '0'
        }
        $workflowUpdateBody = @{
            handle = $workflowHandle
            enabled = $false
            trace = $false
            keepSessions = 0
            meta = @{ name = 'Validación API actualizada'; description = 'Workflow temporal actualizado por el smoke test.' }
            scope = @{}
            steps = @()
            paths = @()
            runAs = '0'
            ownedBy = [string]$createdWorkflow.ownedBy
        }
        if ($createdWorkflow.PSObject.Properties['updatedAt']) {
            $workflowUpdateBody.updatedAt = $createdWorkflow.updatedAt
        }
        $updatedWorkflow = Invoke-AberaApi -Method Put -Path "/api/automation/workflows/$($createdWorkflow.workflowID)" -Body $workflowUpdateBody
        Assert-Abera ($updatedWorkflow.workflowID -eq $createdWorkflow.workflowID) 'La actualización de workflow cambió su ID.'
    } finally {
        if ($null -ne $createdWorkflow) {
            Invoke-AberaApi -Method Delete -Path "/api/automation/workflows/$($createdWorkflow.workflowID)" | Out-Null
        }
        if ($null -ne $createdRecord) {
            Invoke-AberaApi -Method Delete `
                -Path "/api/compose/namespace/$($support.namespaceID)/module/$($problemModule.moduleID)/record/$($createdRecord.recordID)" |
                Out-Null
        }
    }

    Write-Output "demo-api: OK (6 espacios, $moduleTotal módulos, $recordTotal registros, $($workflows.Count) workflows, 12 reportes, CRUD temporal)"
} finally {
    $token = $null
    if ($null -ne $bootstrap -and $null -ne $bootstrap.oauth) {
        $bootstrap.oauth.clientSecret = $null
    }
    $bootstrap = $null
    [GC]::Collect()
}
