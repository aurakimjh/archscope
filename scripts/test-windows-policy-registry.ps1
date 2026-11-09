$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-policy-registry.ps1')
$relative = 'Software\ArchScopePolicyRenameTest-' + [guid]::NewGuid().ToString('N')
$path = 'HKCU:\' + $relative
$child = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($relative + '\Original\Child')
try {
    $child.SetValue('LiteralExpand', '%TEMP%\fixture', [Microsoft.Win32.RegistryValueKind]::ExpandString)
    $acl = [Microsoft.Win32.RegistryAclExtensions]::GetAccessControl($child)
    $acl.SetAccessRuleProtection($true, $true)
    [Microsoft.Win32.RegistryAclExtensions]::SetAccessControl($child, $acl)
    $before = [Microsoft.Win32.RegistryAclExtensions]::GetAccessControl($child).Sddl
} finally { $child.Close() }
try {
    Rename-WindowsPolicyKey -Path ($path + '\Original') -NewName 'Renamed'
    Rename-WindowsPolicyKey -Path ($path + '\Renamed') -NewName 'Original'
    $child = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($relative + '\Original\Child')
    try {
        $after = [Microsoft.Win32.RegistryAclExtensions]::GetAccessControl($child).Sddl
        $literal = $child.GetValue('LiteralExpand', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        if ($before -ne $after -or $literal -ne '%TEMP%\fixture') { throw 'Native rename changed ACL or value data.' }
    } finally { $child.Close() }
    Write-Output 'PASS: native registry rename preserves protected child ACL and literal ExpandString.'
} finally {
    if ($relative -notmatch '^Software\\ArchScopePolicyRenameTest-[0-9a-f]{32}$') { throw 'Unexpected fixture path.' }
    # Delete only these three known, empty-of-subkeys test keys, never a tree.
    [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKey($relative + '\Original\Child', $false)
    [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKey($relative + '\Original', $false)
    [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKey($relative, $false)
}
