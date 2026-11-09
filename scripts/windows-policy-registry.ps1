# RegRenameKey renames the actual registry subtree. PowerShell's registry
# Rename-Item provider can copy/recreate it and change owner/group metadata.
# https://learn.microsoft.com/windows/win32/api/winreg/nf-winreg-regrenamekey
if (-not ('ArchScopeRegistryRename' -as [type])) {
    Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class ArchScopeRegistryRename {
    [DllImport("advapi32.dll", CharSet = CharSet.Unicode)]
    public static extern int RegRenameKey(IntPtr key, string oldName, string newName);
}
'@
}

function Rename-WindowsPolicyKey {
    param([Parameter(Mandatory)][string]$Path, [Parameter(Mandatory)][string]$NewName)
    if ($NewName.Contains('\') -or $NewName.Contains('/') -or -not $NewName) {
        throw 'The new registry name must be a single nonempty key name.'
    }
    if ($Path -notmatch '^(HKLM|HKCU):\\(.+)\\([^\\]+)$') {
        throw 'Expected an absolute HKLM or HKCU provider path.'
    }
    $hive = if ($Matches[1] -eq 'HKLM') { [Microsoft.Win32.RegistryHive]::LocalMachine } else { [Microsoft.Win32.RegistryHive]::CurrentUser }
    $parentPath = $Matches[2]
    $leaf = $Matches[3]
    $base = [Microsoft.Win32.RegistryKey]::OpenBaseKey($hive, [Microsoft.Win32.RegistryView]::Default)
    try {
        $rights = [Security.AccessControl.RegistryRights]::ReadKey -bor [Security.AccessControl.RegistryRights]::WriteKey -bor [Security.AccessControl.RegistryRights]::Delete
        $parent = $base.OpenSubKey($parentPath, [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree, $rights)
        if ($null -eq $parent) { throw 'Registry parent does not exist.' }
        try {
            $result = [ArchScopeRegistryRename]::RegRenameKey($parent.Handle.DangerousGetHandle(), $leaf, $NewName)
            if ($result -ne 0) { throw [ComponentModel.Win32Exception]::new($result) }
        } finally { $parent.Close() }
    } finally { $base.Close() }
}
