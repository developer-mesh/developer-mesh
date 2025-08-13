# Edge MCP Installation Script for Windows
# This script downloads and installs the Edge MCP binary for Windows

param(
    [string]$Version = "",
    [string]$InstallDir = "$env:ProgramFiles\edge-mcp"
)

$ErrorActionPreference = "Stop"

# Configuration
$Repo = "developer-mesh/developer-mesh"
$BinaryName = "edge-mcp.exe"

# Functions
function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Color
}

function Get-Platform {
    $arch = [System.Environment]::Is64BitOperatingSystem
    
    if ($arch) {
        return "windows-amd64"
    } else {
        return "windows-386"
    }
}

function Get-LatestVersion {
    try {
        $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases"
        $edgeMcpRelease = $releases | Where-Object { $_.tag_name -like "edge-mcp-v*" } | Select-Object -First 1
        
        if ($edgeMcpRelease) {
            return $edgeMcpRelease.tag_name -replace "edge-mcp-v", ""
        } else {
            return "nightly"
        }
    } catch {
        Write-ColorOutput "Warning: Could not fetch latest version, using nightly" -Color Yellow
        return "nightly"
    }
}

function Install-EdgeMCP {
    param(
        [string]$Platform,
        [string]$Version
    )
    
    Write-ColorOutput "Installing Edge MCP $Version for $Platform..." -Color Yellow
    
    # Construct download URL
    $binaryFile = "edge-mcp-$Platform.exe"
    $archiveFile = "$binaryFile.zip"
    
    if ($Version -eq "nightly") {
        $downloadUrl = "https://github.com/$Repo/releases/download/edge-mcp-nightly/$binaryFile"
        
        Write-ColorOutput "Downloading nightly build..." -Color Yellow
        try {
            Invoke-WebRequest -Uri $downloadUrl -OutFile $BinaryName -UseBasicParsing
        } catch {
            Write-ColorOutput "Failed to download Edge MCP: $_" -Color Red
            exit 1
        }
    } else {
        $downloadUrl = "https://github.com/$Repo/releases/download/edge-mcp-v$Version/$archiveFile"
        
        Write-ColorOutput "Downloading from: $downloadUrl" -Color Yellow
        try {
            Invoke-WebRequest -Uri $downloadUrl -OutFile $archiveFile -UseBasicParsing
        } catch {
            Write-ColorOutput "Failed to download Edge MCP: $_" -Color Red
            exit 1
        }
        
        Write-ColorOutput "Extracting archive..." -Color Yellow
        try {
            Expand-Archive -Path $archiveFile -DestinationPath . -Force
            Move-Item -Path $binaryFile -Destination $BinaryName -Force
            Remove-Item -Path $archiveFile -Force
        } catch {
            Write-ColorOutput "Failed to extract archive: $_" -Color Red
            exit 1
        }
    }
    
    # Create installation directory
    if (!(Test-Path $InstallDir)) {
        Write-ColorOutput "Creating installation directory: $InstallDir" -Color Yellow
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    # Move to installation directory
    try {
        Move-Item -Path $BinaryName -Destination "$InstallDir\$BinaryName" -Force
        Write-ColorOutput "Edge MCP installed to: $InstallDir\$BinaryName" -Color Green
    } catch {
        Write-ColorOutput "Failed to install Edge MCP: $_" -Color Red
        Write-ColorOutput "You may need to run this script as Administrator" -Color Yellow
        exit 1
    }
}

function Add-ToPath {
    param(
        [string]$Dir
    )
    
    $currentPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine)
    
    if ($currentPath -notlike "*$Dir*") {
        Write-ColorOutput "Adding $Dir to system PATH..." -Color Yellow
        
        try {
            [Environment]::SetEnvironmentVariable(
                "Path",
                "$currentPath;$Dir",
                [EnvironmentVariableTarget]::Machine
            )
            Write-ColorOutput "PATH updated successfully" -Color Green
            Write-ColorOutput "Please restart your terminal for PATH changes to take effect" -Color Yellow
        } catch {
            Write-ColorOutput "Failed to update PATH. Run as Administrator to update system PATH" -Color Yellow
            Write-ColorOutput "You can manually add to PATH or run Edge MCP using full path:" -Color Yellow
            Write-ColorOutput "  $InstallDir\$BinaryName" -Color White
        }
    } else {
        Write-ColorOutput "$Dir is already in PATH" -Color Green
    }
}

function Test-Installation {
    $edgeMcpPath = "$InstallDir\$BinaryName"
    
    if (Test-Path $edgeMcpPath) {
        Write-ColorOutput "`nEdge MCP installed successfully!" -Color Green
        
        # Try to get version
        try {
            $versionOutput = & $edgeMcpPath --version 2>&1
            Write-ColorOutput "Installed version: $versionOutput" -Color Green
        } catch {
            Write-ColorOutput "Installed (version check failed)" -Color Yellow
        }
        
        Write-ColorOutput "`nQuick start:" -Color Yellow
        Write-ColorOutput "  edge-mcp --port 8082           # Run Edge MCP" -Color White
        Write-ColorOutput "  edge-mcp --help                # Show help" -Color White
        Write-ColorOutput "  edge-mcp --version             # Show version" -Color White
        Write-ColorOutput "`nFor IDE setup guides, visit:" -Color Yellow
        Write-ColorOutput "  https://github.com/$Repo/tree/main/apps/edge-mcp/docs/ide-setup" -Color White
    } else {
        Write-ColorOutput "Installation failed!" -Color Red
        exit 1
    }
}

# Main installation flow
function Main {
    Write-ColorOutput "Edge MCP Installer for Windows" -Color Cyan
    Write-ColorOutput "==============================`n" -Color Cyan
    
    # Check if running as admin
    $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
    
    if (!$isAdmin) {
        Write-ColorOutput "Note: Running without Administrator privileges" -Color Yellow
        Write-ColorOutput "Some features (like updating system PATH) may not work`n" -Color Yellow
    }
    
    # Detect platform
    $platform = Get-Platform
    Write-ColorOutput "Detected platform: $platform" -Color Yellow
    
    # Get version
    if ([string]::IsNullOrEmpty($Version)) {
        $Version = Get-LatestVersion
    }
    Write-ColorOutput "Version to install: $Version`n" -Color Yellow
    
    # Create temp directory
    $tempDir = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path $_ }
    Push-Location $tempDir
    
    try {
        # Install Edge MCP
        Install-EdgeMCP -Platform $platform -Version $Version
        
        # Add to PATH if running as admin
        if ($isAdmin) {
            Add-ToPath -Dir $InstallDir
        } else {
            Write-ColorOutput "`nTo add Edge MCP to PATH, run this script as Administrator" -Color Yellow
        }
        
        # Test installation
        Test-Installation
    } finally {
        # Cleanup
        Pop-Location
        Remove-Item -Path $tempDir -Recurse -Force
    }
}

# Run main function
Main