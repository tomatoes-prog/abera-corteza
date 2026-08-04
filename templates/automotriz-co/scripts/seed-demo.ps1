# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

[CmdletBinding()]
param(
    [string]$BaseUrl = 'http://localhost:8080',
    [Parameter(Mandatory = $true)][string]$ClientId,
    [string]$ClientSecret = $env:ABERA_CLIENT_SECRET,
    [string]$Email = $env:ABERA_ADMIN_EMAIL,
    [string]$Password = $env:ABERA_ADMIN_PASSWORD
)

. (Join-Path $PSScriptRoot '..\..\_shared\seed-api.ps1')
Invoke-AberaDemoSeed -TemplateId automotriz-co -BaseUrl $BaseUrl -ClientId $ClientId -ClientSecret $ClientSecret -Email $Email -Password $Password
