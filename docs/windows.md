# Nudge en Windows (beta)

Hay un build para Windows 10/11 x64. Se compila desde Linux y el workflow
[`windows.yml`](../.github/workflows/windows.yml) lo prueba en GitHub Actions
sobre `windows-latest`: corre los tests de Go y prueba el zip de punta a punta
(instalador, servidor, CLI por el socket de control, emparejamiento,
clipboard y desinstalador). **Todavía no lo probé en una PC física con
Windows**, así que puede fallar. Si algo no anda, mirá el
[log](#desinstalar-y-log) y avisá.

## Qué se descarga

Desde la [página de releases](https://github.com/Scag95/Nudge/releases), cada
uno con su `.sha256`:

- `nudge-setup-<versión>-windows-x64.exe`: el instalador (recomendado).
- `nudge-<versión>-windows-x64.zip`: lo mismo con un instalador por scripts,
  como alternativa.

Los dos instalan dos ejecutables del mismo código:

- `nudge.exe`: consola. Es el CLI `nudge pair` y, abierto a mano, el servidor
  con el log en pantalla.
- `nudge-background.exe`: sin ventana. Es el servidor en segundo plano.

Además vienen el ícono, los scripts que usan el instalador y la bandeja
(`toggle-autostart.ps1`, `firewall.ps1`) y `README-windows.txt`.

## Instalar

### Con el instalador (recomendado)

Ejecutá `nudge-setup-<versión>-windows-x64.exe`. No pide administrador:
instala en `%LOCALAPPDATA%\Programs\Nudge`, crea **Nudge** en el menú Inicio
(carpeta Nudge) y al final ofrece "Iniciar Nudge ahora". Tiene tres opciones,
marcadas por defecto:

- **Iniciar Nudge al iniciar sesión** (acceso directo en la carpeta Inicio).
- **Agregar nudge al PATH del usuario** (para `nudge pair` en una terminal
  nueva).
- **Permitir Nudge en el Firewall de Windows**: pide permiso de
  administrador (UAC) para crear las reglas (ver [Firewall](#firewall)). Si
  ya existen, no pregunta; si rechazás el aviso, la instalación sigue igual.

Volver a ejecutarlo actualiza en el lugar: detiene Nudge, reemplaza los
archivos, recuerda las opciones de la vez anterior (una opción desmarcada
quita el inicio automático o la entrada del `PATH`) y vuelve a iniciarlo.
También actualiza una instalación hecha con el zip (misma carpeta) y borra
sus accesos directos viejos ("Nudge - autostart on-off", "Uninstall Nudge").

Instalación sin ventanas (por ejemplo, para scripts):

```bat
nudge-setup-1.0.0-windows-x64.exe /VERYSILENT /SUPPRESSMSGBOXES /TASKS="autostart,path"
```

Sin `/TASKS` usa las opciones por defecto, incluida la del firewall, y el
aviso de administrador aparece igual.

### Con el zip (alternativa)

1. Extraé el zip completo (clic derecho → *Extraer todo*).
2. Doble clic en `Install Nudge.cmd`. No pide administrador: instala en la
   misma carpeta, crea la carpeta **Nudge** en el menú Inicio ("Nudge",
   "Nudge - autostart on-off", "Uninstall Nudge"), activa el inicio
   automático, agrega la carpeta al `PATH` del usuario e inicia el servidor.
   Volver a ejecutarlo actualiza en el lugar.

Opciones de `install.ps1`: `-NoAutostart`, `-FirewallRule yes|no` (para
instalar sin preguntas), `-NoStart`, `-NoPause`.

### SmartScreen

Ni el instalador ni los ejecutables **están firmados**: SmartScreen puede
mostrar "Windows protegió su PC" (*Más información* → *Ejecutar de todas
formas*). Verificá antes el SHA256 (`certutil -hashfile <archivo> SHA256`)
contra el `.sha256` de la release.

## Emparejar

Cuando un teléfono pide emparejarse, el ícono de la bandeja muestra una
notificación ("Solicitud de emparejamiento") y aparece una ventana **Sí/No**
con su nombre, su IP y el código de verificación de la PC
(`Código: XXXX-XXXX-XXXX`), con **No** como botón por defecto (un Enter sin
querer rechaza). Elegí **Sí** solo si el teléfono muestra el mismo código.
La ventana es el único lugar donde se aprueba. El CLI también sirve, desde
una terminal nueva: `nudge pair list` (la primera línea es
`Verification code: ...`) y `nudge pair --last`.

El código también está siempre en el menú del ícono de la bandeja (ver
abajo). Un teléfono que la PC ya conocía solo se reconecta sin ventana
Sí/No si presenta su token vigente por la conexión cifrada; si no lo tiene
(por ejemplo, después del aviso "El certificado de la PC cambió"), vuelve a
aparecer la ventana Sí/No con el código, como para un teléfono nuevo. Si
una PC con una versión anterior de Nudge igual le da acceso sin ventana,
el teléfono pregunta "¿La PC muestra el código …?": tocá **Sí, coincide**
solo si es el mismo que ves en la bandeja o con `nudge pair list`. Con
**No** el teléfono no guarda nada y cancela el emparejamiento.

El tráfico con el teléfono va cifrado (HTTPS/WSS) con un certificado
autofirmado que el servidor guarda en `%APPDATA%\nudge\tls.crt` y
`%APPDATA%\nudge\tls.key`; el teléfono lo recuerda al emparejar. Si borrás
`tls.key` (o `%APPDATA%\nudge`, por ejemplo al desinstalar con borrado de
datos), el servidor genera otro y **cada teléfono tiene que emparejarse de
nuevo**.

## Ícono en la bandeja

Mientras el servidor corre, Nudge tiene un ícono en el área de notificación.
Con clic derecho muestra el estado ("Escuchando en el puerto 8765"), el
código de verificación de la PC (`Código: XXXX-XXXX-XXXX`) y los teléfonos
emparejados con su IP ("Ningún teléfono emparejado" si no hay).
Es solo informativo: para revocar un teléfono usá
`nudge pair revoke --device <id>` (el id sale en `nudge pair list`). Si
Windows esconde el ícono, está en la flechita "Mostrar iconos ocultos". Si
el explorador se reinicia, el ícono vuelve solo.

## Inicio automático

El menú del ícono de la bandeja (o, con el zip, "Nudge - autostart on-off" en
el menú Inicio) activa o desactiva el acceso directo de la carpeta Inicio
del usuario (también `toggle-autostart.ps1 on|off|status`). No inicia ni detiene el servidor que ya
corre.

## Firewall

El instalador explica y pregunta si agregar dos reglas de entrada, solo en
redes **privadas** y solo para `nudge-background.exe`: "Nudge" (TCP 8765) y
"Nudge mDNS" (UDP 5353). Hace falta aceptar el aviso de administrador (UAC);
si lo rechazás, la instalación sigue y podés volver a ejecutarla después. La
WiFi tiene que estar marcada como red privada.

El instalador `.exe` le pasa los comandos al PowerShell elevado en línea
(`-EncodedCommand`), con las mismas reglas que `firewall.ps1`: nunca ejecuta
como administrador un archivo de la carpeta de instalación, que el usuario puede
modificar. `firewall.ps1` queda para la instalación con el zip.

La ruta de instalación (que incluye el nombre de usuario) viaja dentro de ese comando
como base64 y se decodifica en PowerShell, sin comillas que puedan romperse. La
codificación se prueba con `scripts/test-windows-setup.sh` (necesita Inno Setup).

## Desinstalar y log

Con el instalador: Configuración → Aplicaciones → Nudge → *Desinstalar*. Con
el zip: menú Inicio → Nudge → "Uninstall Nudge" (o `Uninstall Nudge.cmd`).
Los dos detienen el servidor y borran los accesos directos, la entrada del
`PATH`, las reglas del firewall (pide administrador solo si existen) y la
carpeta de instalación. Preguntan antes de borrar `%APPDATA%\nudge` (por
defecto No), donde quedan los teléfonos emparejados (`state.json`).

El servidor en segundo plano escribe su log en `%APPDATA%\nudge\nudge.log`.

## Límites

- Probado solo en GitHub Actions (`windows-latest`), no en una PC física.
- No hay panel como en Linux: el ícono de la bandeja solo informa; aprobar es
  con la ventana Sí/No y revocar con `nudge pair revoke`.
- Ejecutables sin firmar (SmartScreen).

## Recompilar el zip y el instalador

El zip, desde Linux, con Go 1.24, `rsvg-convert`, ImageMagick (`magick`) y
`zip`:

```bash
scripts/build-windows.sh 0.1.0
# salida: dist/nudge-0.1.0-windows-x64.zip y su .sha256
```

El instalador, en Windows (Git Bash) con
[Inno Setup 6](https://jrsoftware.org/isinfo.php)
(`winget install JRSoftware.InnoSetup`), a partir de ese zip o de una carpeta
con su contenido:

```bash
scripts/build-windows-setup.sh            # usa dist/nudge-<versión>-windows-x64.zip
scripts/build-windows-setup.sh <carpeta>  # o los archivos ya extraídos
# salida: dist/nudge-setup-<versión>-windows-x64.exe y su .sha256
```

La versión sale de `cmd/server/version.go`; el script del instalador es
[`packaging/windows/nudge.iss`](../packaging/windows/nudge.iss).

Para compilar solo el `.exe`, ver
[Compilar los binarios](development.md#compilar-los-binarios).
