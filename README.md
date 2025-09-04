# TP0: Docker + Comunicaciones + Concurrencia
## Mateo Alvarez - 108666
### Facultad de Ingeniería - 75.74 Sistemas Distribuidos I

En el presente repositorio se provee una implementación básica de una arquitectura cliente/servidor aplicada, en donde todas las dependencias del mismo se encuentran encapsuladas en containers. Se pueden distinguir 8 ramas que aluden a ejercicios incrementales que culminan en la creación de una aplicación de lotería centralizada en un servidor.

# Protocolo de Comunicación:
El protocolo implementado es basado en texto plano con encoding UTF-8, lo que facilita el debugging y garantiza compatibilidad cross-platform. Utiliza delimitadores jerárquicos para estructurar la información. Los mensajes **se definen siempre en dos líneas: un header y un body**.  

- El **header** indica el tipo de mensaje y suele incluir un contador (`S:<n>`, `R:1`, `W:<k>`, `F:1`).  
- El **body** contiene los datos (apuestas serializadas, DNIs de ganadores, “OK/FAIL”, “FINISHED”, o “N”).  

Esto simplifica el parsing en cliente y servidor: todos esperan exactamente dos líneas por mensaje.

---

### Mensajes Cliente → Servidor

- **Bets**  
  ```
  S:<AMOUNT>
  bet1~bet2~...~betN
  ```
  El header indica el inicio de un lote de apuestas.  
  `<AMOUNT>` especifica cuántas apuestas contiene el lote.  
  El payload (`body`) contiene las apuestas serializadas, separadas por `~`.  
  Cada apuesta se codifica como:  
  `agency;nombre;apellido;dni;fecha_nacimiento;numero_apostado`

- **FINISHED**  
  ```
  F:1
  FINISHED
  ```
  Mensaje de control que notifica al servidor que el cliente terminó de enviar todas sus apuestas.  
  Dispara la barrera de sincronización en el servidor.

---

### Mensajes Servidor → Cliente

- **ACK (éxito)**  
  ```
  R:1
  OK
  ```
  Confirmación de que un lote fue procesado y almacenado correctamente.

- **ACK (error)**  
  ```
  R:1
  FAIL
  ```
  Indica que ocurrió un error procesando el lote.

- **WINNERS**  
  ```
  W:<COUNT>
  dni1~dni2~...~dniK
  ```
  Lista de DNIs ganadores para la agencia del cliente.  
  `<COUNT>` indica la cantidad de DNIs en el body.

- **SIN GANADORES**  
  ```
  W:0
  N
  ```
  Se usa para agencias sin ganadores, evitando mensajes vacíos.

---

### Lista de Delimitadores del Protocolo

- **"\n" (newline)** – separa header y body, y marca fin de cada mensaje.  
- **"~"** – separa múltiples apuestas dentro de un lote, o múltiples DNIs en la respuesta de ganadores.  
- **";"** – separa los campos individuales dentro de cada apuesta (`agency;nombre;apellido;dni;fecha;numero`).  
- **":"** – separa el prefijo del valor en el header (`S:3`, `W:2`, etc.).

---

## Flujo:

### Fase 1: Handshake y Conexión
El cliente establece conexión TCP con el servidor en el puerto 12345. La conexión se mantiene durante toda la sesión.

### Fase 2: Envío de Lotes de Apuestas
Ejemplo:
```
S:3
1;Juan;Perez;12345678;1990-01-01;1234~1;Maria;Lopez;87654321;1985-02-02;5678~1;Carlos;Garcia;11111111;1980-03-03;9012
```
Respuesta:
```
R:1
OK
```

### Fase 3: Notificación de Finalización
Cliente envía:
```
F:1
FINISHED
```
El servidor espera a que todos los clientes lleguen a este punto (barrera).

### Fase 4: Respuesta de Ganadores
Servidor responde:
- Caso A (con ganadores):
  ```
  W:3
  12345678~87654321~11111111
  ```
- Caso B (sin ganadores):
  ```
  W:0
  N
  ```

---

# Concurrencia y Sincronización

## Sincronización entre threads - Barreras
El sistema utiliza un Threading Barrier como mecanismo principal de sincronización para coordinar la ejecución del sorteo entre múltiples agencias. La barrera se inicializa en el servidor con el número exacto de clientes esperados, creando un punto de encuentro obligatorio donde todos los threads deben llegar antes de proceder con el sorteo. Cuando cada cliente completa el envío de todas sus apuestas, envía un mensaje FINISHED al servidor, lo que hace que su thread correspondiente llame al método wait() de la barrera. Este thread queda bloqueado hasta que todos los demás threads de cliente lleguen al mismo punto de sincronización. Una vez que el último cliente alcanza la barrera, todos los threads se desbloquean simultáneamente, garantizando que el sorteo se ejecute únicamente cuando todas las apuestas hayan sido procesadas y almacenadas.

La barrera proporciona garantías críticas para la integridad del sistema: asegura que el sorteo ocurre exactamente una vez, previene condiciones de carrera en el acceso al archivo de apuestas, y garantiza que ningún cliente reciba resultados antes de que se complete el procesamiento global. El thread que obtiene el índice 0 al cruzar la barrera tiene la responsabilidad exclusiva de loggear el evento, mientras que los demás threads procesan inmediatamente los ganadores específicos de su agencia. Para prevenir deadlocks, la barrera incluye un timeout configurable de 120 segundos que permite al sistema recuperarse en caso de que algún cliente falle o se desconecte inesperadamente. Si se alcanza el timeout, los threads activos reciben una excepción que les permite manejar la situación de error de manera controlada, loggeando el problema y liberando recursos apropiadamente.

### Manejo de acceso compartido a Archivos - Locks
Además de la barrera, el sistema utiliza un **Lock** (mutua exclusión) para proteger el acceso concurrente al archivo de almacenamiento de apuestas (`bets.csv`). Dado que múltiples threads pueden intentar escribir apuestas al mismo tiempo, se emplea un `threading.Lock` que se adquiere antes de abrir y escribir en el archivo, y se libera inmediatamente después de finalizar la operación.  

Este mecanismo garantiza que las operaciones de I/O sean atómicas y que las filas del CSV no se corrompan por escrituras intercaladas. Sin el uso de locks, podría ocurrir que dos hilos intenten escribir en el archivo simultáneamente, provocando pérdida o mezcla de datos.  


## Instrucciones de uso
El repositorio cuenta con un **Makefile** que incluye distintos comandos en forma de targets. Los targets se ejecutan mediante la invocación de:  **make \<target\>**. Los target imprescindibles para iniciar y detener el sistema son **docker-compose-up** y **docker-compose-down**, siendo los restantes targets de utilidad para el proceso de depuración.

Los targets disponibles son:

| target  | accion  |
|---|---|
|  `docker-compose-up`  | Inicializa el ambiente de desarrollo. Construye las imágenes del cliente y el servidor, inicializa los recursos a utilizar (volúmenes, redes, etc) e inicia los propios containers. |
| `docker-compose-down`  | Ejecuta `docker-compose stop` para detener los containers asociados al compose y luego  `docker-compose down` para destruir todos los recursos asociados al proyecto que fueron inicializados. Se recomienda ejecutar este comando al finalizar cada ejecución para evitar que el disco de la máquina host se llene de versiones de desarrollo y recursos sin liberar. |
|  `docker-compose-logs` | Permite ver los logs actuales del proyecto. Acompañar con `grep` para lograr ver mensajes de una aplicación específica dentro del compose. |
| `docker-image`  | Construye las imágenes a ser utilizadas tanto en el servidor como en el cliente. Este target es utilizado por **docker-compose-up**, por lo cual se lo puede utilizar para probar nuevos cambios en las imágenes antes de arrancar el proyecto. |
| `build` | Compila la aplicación cliente para ejecución en el _host_ en lugar de en Docker. De este modo la compilación es mucho más veloz, pero requiere contar con todo el entorno de Golang y Python instalados en la máquina _host_. |

## Parte 1: Introducción a Docker

### Ejercicio N°1:
Definir un script de bash `generar-compose.sh` que permita crear una definición de Docker Compose con una cantidad configurable de clientes.  El nombre de los containers deberá seguir el formato propuesto: client1, client2, client3, etc. 

### Resolución:
El script deberá ubicarse está en la raíz del proyecto y recibe por parámetro el nombre del archivo de salida y la cantidad de clientes esperados:
Decidí hacerlo en bash para consolidar conocimientos de hacer scripts, es bastante simple y se extendió todos los ejercicios:
```
#!/bin/bash
echo "Nombre del archivo de salida: $1"
echo "Cantidad de clientes: $2"

if [[ "${1:-}" == "" || "${2:-}" == "" ]]; then
  echo "Uso: $0 <archivo_salida> <cantidad_clientes>" >&2
  exit 1
fi

OUTPUT_FILE="$1"
CLIENT_NUMBER="$2"

cat > "$OUTPUT_FILE" <<'YAML'
name: tp0
services:
  server:
    container_name: server
    image: server:latest
    entrypoint: python3 /main.py
    environment:
      - PYTHONUNBUFFERED=1
      - LOGGING_LEVEL=DEBUG
    networks:
      - testing_net
YAML


for ((i=1; i<=CLIENT_NUMBER; i++)); do 
cat >> "$OUTPUT_FILE" <<YAML
  client${i}:
    container_name: client${i}
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID=${i}
      - CLI_LOG_LEVEL=DEBUG
    networks:
      - testing_net
    depends_on:
      - server
YAML
done


# Footer con la red
cat >> "$OUTPUT_FILE" <<'YAML'
networks:
  testing_net:
    ipam:
      driver: default
      config:
        - subnet: 172.25.125.0/24
YAML
```

### Ejercicio N°2:
Modificar el cliente y el servidor para lograr que realizar cambios en el archivo de configuración no requiera reconstruír las imágenes de Docker para que los mismos sean efectivos. La configuración a través del archivo correspondiente (`config.ini` y `config.yaml`, dependiendo de la aplicación) debe ser inyectada en el container y persistida por fuera de la imagen (hint: `docker volumes`).

### Resolución:
Se agregaron los volumenes al script de generar compose con la sintaxis:
```
    volumes:
      - ./server/config.ini:/config.ini
      
    volumes:
      - ./client/config.yaml:/config.yaml
``` 
A su vez se corrigieron los Dockerfiles para no copiar estas rutas y poder almacenar los volúmenes.

### Ejercicio N°3:
Crear un script de bash `validar-echo-server.sh` que permita verificar el correcto funcionamiento del servidor utilizando el comando `netcat` para interactuar con el mismo. Dado que el servidor es un echo server, se debe enviar un mensaje al servidor y esperar recibir el mismo mensaje enviado.

### Resolución:
Ninguna magia acá, es un script que declara variables:
- SERVER_PORT: puerto donde escucha el servidor.
- SERVER_IP: nombre o IP del contenedor que corre el servidor.
- NETWORK_NAME: red de Docker donde están conectados cliente y servidor.
- MESSAGE: el texto de prueba que se va a enviar al servidor.
Luego, el script levanta un contenedor efímero con la imagen liviana Alpine, gracias a la opción --rm que hace que desaparezca al terminar. Ese contenedor se conecta a la red de Docker definida para las pruebas, tp0_testing_net, de modo que pueda hablar con el servidor. Una vez adentro del contenedor se ejecuta un comando con /bin/sh -c, donde primero se arma el mensaje a enviar usando echo '${MESSAGE}' y enseguida se lo pasa a nc (netcat), que abre la conexión hacia el servidor en la dirección y puerto configurados (SERVER_IP:SERVER_PORT). La salida de esa comunicación, es decir lo que el servidor devuelve, queda almacenada en la variable RESPONSE para luego ser comparada con el mensaje original.
``` 
#!/bin/bash

SERVER_PORT=12345
SERVER_IP=server
# SERVER_LISTEN_BACKLOG=5
# LOGGING_LEVEL=INFO
NETWORK_NAME="tp0_testing_net"
MESSAGE="Hola Mate!!!1"

RESPONSE=$(docker run --rm --network=${NETWORK_NAME} alpine /bin/sh -c "echo '${MESSAGE}' | nc ${SERVER_IP} ${SERVER_PORT}")

if [ "${RESPONSE}" = "${MESSAGE}" ]; then
  echo "action: test_echo_server | result: success"
else
  echo "action: test_echo_server | result: fail"
fi
``` 

### Ejercicio N°4:
Modificar servidor y cliente para que ambos sistemas terminen de forma _graceful_ al recibir la signal SIGTERM. Terminar la aplicación de forma _graceful_ implica que todos los _file descriptors_ (entre los que se encuentran archivos, sockets, threads y procesos) deben cerrarse correctamente antes que el thread de la aplicación principal muera. Loguear mensajes en el cierre de cada recurso (hint: Verificar que hace el flag `-t` utilizado en el comando `docker compose down`).

## Resolución:


``` 
    def _begin_shutdown(self, signum, frame):
        """
        Handle shutdown signal

        If the server receives a SIGTERM signal, this handler ensures it
        starts the shutdown process.
        """
        logging.info("action: sigterm_received | result: success")
        self._running = False
        if self._server_socket:
            self._server_socket.close()

    def __graceful_shutdown(self):
        """Wait for all client threads to finish"""
        logging.info("action: shutdown | result: in_progress")
        
        # Stop accepting new connections
        self._running = False
        
        # Close server socket if not already closed
        try:
            self._server_socket.close()
        except:
            pass
            
        # Wait for all client threads to complete their work
        active_threads = [t for t in self.client_threads if t.is_alive()]
        if active_threads:
            logging.info(f"action: waiting_for_threads | count: {len(active_threads)}")
            
            for thread in active_threads:
                thread.join(timeout=30)  # Wait max 30 seconds per thread
                if thread.is_alive():
                    logging.warning("action: thread_timeout | result: warning")

        logging.info("action: server_shutdown | result: success")
``` 

## Resolución:
El servidor implementa un mecanismo para terminar de forma graceful al recibir SIGTERM, definiendo un handler (_begin_shutdown) que detiene el loop principal y cierra el socket del servidor. Además, tiene una función (__graceful_shutdown) que libera todos los sockets abiertos, tanto del servidor como de los clientes, y loguea el cierre de cada recurso. Esto cumple con el requisito de liberar los file descriptors antes de que termine el thread principal. Para cumplir completamente, debe asegurarse de registrar el handler de SIGTERM en el main y cerrar otros recursos (archivos, threads, procesos) si existen.
``` 
    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """

        # Connection arrived
        logging.info('action: accept_connections | result: in_progress')
        c, addr = self._server_socket.accept()
        logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
        return c

    def _begin_shutdown(self, signum, frame):
        """
        Handle shutdown signal

        If the server receives a SIGTERM signal, this handler ensures it
        starts the shutdown process.
        """
        logging.info("action: sigterm_received | result: success")
        self._running = False
        if self._server_socket:
            self._server_socket.close()

    def __graceful_shutdown(self):
        """
        This function is called when the server is shutting down.
        It ensures all resources are released properly.
        """
        logging.info("action: server_shutdown | result: in_progress")

        try:
            if self._server_socket:
                self._server_socket.close()
        except:
            pass

        for sock in self.__client_socks:
            try:
                logging.info("action: close_client_socket | result: success")
                sock.close()
            except:
                pass

        logging.info("action: server_shutdown | result: success")
``` 

## Parte 2 y 3 Repaso de Comunicaciones y Concurrencia
### Flujo Cliente/Servidor

El flujo del sistema se desarrolla en tres etapas principales:

1. **Envío de apuestas:**  
   Cada cliente (agencia) obtiene sus datos desde variables de entorno o archivos CSV y los serializa en mensajes siguiendo el protocolo definido. Estos mensajes se transmiten al servidor mediante sockets TCP.

2. **Confirmación del servidor:**  
   El servidor recibe las apuestas, las valida y las persiste utilizando las funciones provistas (`store_bet(...)`). Tras cada operación exitosa, responde con un mensaje de confirmación (OK), o con FAIL en caso de error. De esta manera el cliente puede continuar o registrar un fallo.

3. **Finalización y sorteo:**  
   Una vez que los clientes notifican la finalización (FINISHED), el servidor activa la barrera de sincronización. Solo cuando todas las agencias han concluido, se ejecuta el sorteo, y el servidor responde a cada cliente con los resultados de su agencia. Así se evita entregar información parcial.

---

### Arquitectura Cliente/Servidor

## **Cliente (Go):**  
Implementa la lógica de una agencia de quiniela. Se encarga de preparar los datos, serializarlos, enviarlos en *batches* y recibir confirmaciones del servidor. Al finalizar, consulta los resultados de su agencia. Mantiene separada la lógica de dominio (apuestas) de la capa de comunicación (sockets y protocolo).

### Componentes del Cliente (GoLang)

1. **Struct `Bet`**  
   - Representa una apuesta individual.  
   - Contiene los campos: `Agency`, `Name`, `LastName`, `Document`, `Birthdate`, `Number`.  
   - Ofrece métodos de utilidad:  
     - `Serialize()`: convierte la apuesta en un string siguiendo el protocolo.  
     - `DeserializeBet()`: permite reconstruir una apuesta desde un string.  
     - `IsValid()`: valida que los campos esenciales estén presentes.

2. **Struct `BatchReader`**  
   - Responsable de leer las apuestas desde un archivo CSV asociado a la agencia.  
   - Maneja la lectura de forma incremental, devolviendo *batches* de apuestas hasta llegar al EOF.  
   - Mantiene estadísticas internas (`lineNumber`, `totalRead` para manejar la lectura.  
   - Implementa:  
     - `ReadNextBatch()`: devuelve la siguiente tanda de apuestas.  
     - `SerializeBatch()`: serializa un conjunto de apuestas para transmisión.  
     - `Close()`: cierra el descriptor de archivo para evitar fugas de recursos.

3. **Configuración del Cliente (`ClientConfig`)**  
   - Define los parámetros principales de ejecución:  
     - `ServerAddress`: dirección del servidor.  
     - `ID`: identificador de la agencia.  
     - `MessageProtocol`: configuración del protocolo (batch size, delimitadores, mensajes de control).  
   - Se obtiene a través de constantes en el archivo main.go  
   - Permite flexibilidad sin modificar el código fuente, asegurando portabilidad y repetibilidad de experimentos.

## **Servidor (Python):**  
Emula a la central de Lotería Nacional. Administra múltiples conexiones en paralelo con *multithreading*, garantizando consistencia mediante un `Lock` global en operaciones de archivo y una `Barrier` para sincronizar el sorteo. Responde a cada cliente según su agencia, sin hacer broadcasts generales, asegurando escalabilidad y coherencia.
Esta arquitectura asegura robustez en la comunicación, correcta concurrencia en el servidor y simplicidad en el parsing de mensajes, dado que todo se organiza en mensajes de dos líneas (header + body).

### Componentes del Servidor (Python)

1. **Clase `Server`**
   - Expone el ciclo de vida del servidor: `__init__`, `run()`, `_begin_shutdown()`, `__graceful_shutdown()`.
   - Acepta conexiones TCP (`socket.accept`) y crea un thread por cliente (`__handle_client_connection`).
   - Mantiene una lista de threads activos y limpia terminados (`__cleanup_finished_threads`).

2. **Modelo de Concurrencia**
   - **Multithreading**: un hilo por conexión para procesar mensajes en paralelo.
   - **`threading.Lock` global (`_FILE_LOCK`)**: serializa secciones críticas de E/S sobre `bets.csv`.
     - Protege **escritura** (`store_bets(...)`) y **lectura** de ganadores (`load_winning_bets(...)`).
   - **`threading.Barrier` (`lottery_barrier`)**: sincroniza el **sorteo**; todos los clientes deben enviar `FINISHED` para continuar.
     - El thread con índice `0` registra `action: sorteo | result: success`.

3. **Capa de Comunicación (Protocolo de dos líneas)**
   - **Recepción**: `__receive_two_lines()` y helper `__receive_line()` ensamblan siempre **header + body**.
     - Batch: `"S:<n>"` + `"bet1~...~betN"`.
     - Fin de envío: `"F:1"` + `"FINISHED"`.
   - **Envío**: `__send_message(header, body)` garantiza **short-write safe** (loop hasta enviar todos los bytes).
     - Respuesta de confirmación: `"R:1"` + `"OK"/"FAIL"`.
     - Ganadores: `"W:<k>"` + `"dni1~...~dniK"` o `"W:0"` + `"N"`.

4. **Lógica de Negocio**
   - **Batches**: deserializa con `deserialize_batch(...)`, persiste con `store_bets(...)` (dentro de `with _FILE_LOCK:`).
   - **FINISHED + Sorteo**: `__handle_finished_and_return_winners(...)` espera en la **Barrier**, luego obtiene DNIs con `load_winning_bets( agency)` (también bajo lock) y responde a **cada** cliente **solo** con sus ganadores (sin broadcast).

5. **Persistencia y Utilidades (`utils.py`)**
   - Tipos y helpers: `Bet`, `deserialize_bet(...)`, `deserialize_batch(...)`.
   - E/S CSV: `store_bets(...)`, `load_winning_bets(...)` (lectura “streaming”, sin cargar todo en memoria).

6. **Observabilidad y Robustez**
   - **Logging estructurado** (acciones/resultados) para trazabilidad: `apuesta_recibida`, `winners_sent`, `sorteo`, errores de cliente, etc.
   - **Manejo de errores**: try/except a nivel de conexión; ante fallas responde `"R:1\nFAIL\n"` en lugar de cortar con EOF (mejor UX que el silencio… salvo que el silencio sea dorado).
   - **Graceful shutdown**: handler de `SIGTERM` (`_begin_shutdown`) cierra el socket de escucha y `__graceful_shutdown()` hace `join` de threads.
   - **Fds cerrados**: `client_sock.close()` en `finally`, cierre explícito de server socket y archivos protegidos por lock.

7. **Configuración**
   - Parámetros leídos de **env/config** (puerto, backlog, logging level, cantidad esperada de agencias vía `CLI_CLIENTS`).
   - Límite de espera en Barrier (`timeout=120`) para evitar deadlocks si un cliente falla.
