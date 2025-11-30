; 种子解析工具 - 可编译版（修复所有语法错误）
#define MyAppName "种子解析工具"
#define MyAppVersion "1.5"
#define MyAppExeName "SeedParser.exe"
#define MyAppAssocName MyAppName + " File"
#define MyAppAssocExt ".myp"
#define MyAppAssocKey StringChange(MyAppAssocName, " ", "") + MyAppAssocExt

[Setup]
; 1. 修复：AppId 闭合花括号（必做）
AppId={{D2730039-3EB8-410E-BAFB-40FBC4A92601}}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
DefaultDirName={autopf}\{#MyAppName}
UninstallDisplayIcon={app}\{#MyAppExeName}
; 2. 修复：64位架构配置（Inno Setup 6 正确值）
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
ChangesAssociations=yes
DisableProgramGroupPage=yes
; 3. 可选：注释许可证文件（先测试编译，后续再恢复）
; LicenseFile=D:\goprojact\SeedParser\使用条款.txt
OutputDir=D:\goprojact\SeedParser\goujian
OutputBaseFilename=种子解析工具
SolidCompression=yes
; 4. 修复：WizardStyle 有效值
WizardStyle=modern

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
; 中文语言包（需文件存在时取消注释）
; Name: "chinesesimplified"; MessagesFile: "compiler:ChineseSimplified.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
; 主程序（确保路径存在）
Source: "D:\goprojact\SeedParser\build\bin\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
; tools 文件夹（完整复制）
Source: "D:\goprojact\SeedParser\build\bin\tools\*"; DestDir: "{app}\tools"; Flags: ignoreversion recursesubdirs createallsubdirs

[Registry]
; 文件关联（保留原有逻辑）
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocExt}\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Flags: uninsdeletevalue
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocKey}"; ValueType: string; ValueName: ""; ValueData: "{#MyAppAssocName}"; Flags: uninsdeletekey
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocKey}\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\{#MyAppExeName},0"
Root: HKA; Subkey: "Software\Classes\{#MyAppAssocKey}\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent