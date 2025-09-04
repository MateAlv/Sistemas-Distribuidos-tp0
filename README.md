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

# Sincronización

## Implementación de la Barrera de Sincronización
El sistema utiliza un Threading Barrier como mecanismo principal de sincronización para coordinar la ejecución del sorteo entre múltiples agencias. La barrera se inicializa en el servidor con el número exacto de clientes esperados, creando un punto de encuentro obligatorio donde todos los threads deben llegar antes de proceder con el sorteo. Cuando cada cliente completa el envío de todas sus apuestas, envía un mensaje FINISHED al servidor, lo que hace que su thread correspondiente llame al método wait() de la barrera. Este thread queda bloqueado hasta que todos los demás threads de cliente lleguen al mismo punto de sincronización. Una vez que el último cliente alcanza la barrera, todos los threads se desbloquean simultáneamente, garantizando que el sorteo se ejecute únicamente cuando todas las apuestas hayan sido procesadas y almacenadas.

La barrera proporciona garantías críticas para la integridad del sistema: asegura que el sorteo ocurre exactamente una vez, previene condiciones de carrera en el acceso al archivo de apuestas, y garantiza que ningún cliente reciba resultados antes de que se complete el procesamiento global. El thread que obtiene el índice 0 al cruzar la barrera tiene la responsabilidad exclusiva de loggear el evento, mientras que los demás threads procesan inmediatamente los ganadores específicos de su agencia. Para prevenir deadlocks, la barrera incluye un timeout configurable de 120 segundos que permite al sistema recuperarse en caso de que algún cliente falle o se desconecte inesperadamente. Si se alcanza el timeout, los threads activos reciben una excepción que les permite manejar la situación de error de manera controlada, loggeando el problema y liberando recursos apropiadamente.

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

## Parte 2: Repaso de Comunicaciones

Las secciones de repaso del trabajo práctico plantean un caso de uso denominado **Lotería Nacional**. Para la resolución de las mismas deberá utilizarse como base el código fuente provisto en la primera parte, con las modificaciones agregadas en el ejercicio 4.

### Ejercicio N°5:
Modificar la lógica de negocio tanto de los clientes como del servidor para nuestro nuevo caso de uso.

#### Cliente
Emulará a una _agencia de quiniela_ que participa del proyecto. Existen 5 agencias. Deberán recibir como variables de entorno los campos que representan la apuesta de una persona: nombre, apellido, DNI, nacimiento, numero apostado (en adelante 'número'). Ej.: `NOMBRE=Santiago Lionel`, `APELLIDO=Lorca`, `DOCUMENTO=30904465`, `NACIMIENTO=1999-03-17` y `NUMERO=7574` respectivamente.

Los campos deben enviarse al servidor para dejar registro de la apuesta. Al recibir la confirmación del servidor se debe imprimir por log: `action: apuesta_enviada | result: success | dni: ${DNI} | numero: ${NUMERO}`.



#### Servidor
Emulará a la _central de Lotería Nacional_. Deberá recibir los campos de la cada apuesta desde los clientes y almacenar la información mediante la función `store_bet(...)` para control futuro de ganadores. La función `store_bet(...)` es provista por la cátedra y no podrá ser modificada por el alumno.
Al persistir se debe imprimir por log: `action: apuesta_almacenada | result: success | dni: ${DNI} | numero: ${NUMERO}`.

#### Comunicación:
Se deberá implementar un módulo de comunicación entre el cliente y el servidor donde se maneje el envío y la recepción de los paquetes, el cual se espera que contemple:
* Definición de un protocolo para el envío de los mensajes.
* Serialización de los datos.
* Correcta separación de responsabilidades entre modelo de dominio y capa de comunicación.
* Correcto empleo de sockets, incluyendo manejo de errores y evitando los fenómenos conocidos como [_short read y short write_](https://cs61.seas.harvard.edu/site/2018/FileDescriptors/).


### Ejercicio N°6:
Modificar los clientes para que envíen varias apuestas a la vez (modalidad conocida como procesamiento por _chunks_ o _batchs_). 
Los _batchs_ permiten que el cliente registre varias apuestas en una misma consulta, acortando tiempos de transmisión y procesamiento.

La información de cada agencia será simulada por la ingesta de su archivo numerado correspondiente, provisto por la cátedra dentro de `.data/datasets.zip`.
Los archivos deberán ser inyectados en los containers correspondientes y persistido por fuera de la imagen (hint: `docker volumes`), manteniendo la convencion de que el cliente N utilizara el archivo de apuestas `.data/agency-{N}.csv` .

En el servidor, si todas las apuestas del *batch* fueron procesadas correctamente, imprimir por log: `action: apuesta_recibida | result: success | cantidad: ${CANTIDAD_DE_APUESTAS}`. En caso de detectar un error con alguna de las apuestas, debe responder con un código de error a elección e imprimir: `action: apuesta_recibida | result: fail | cantidad: ${CANTIDAD_DE_APUESTAS}`.

La cantidad máxima de apuestas dentro de cada _batch_ debe ser configurable desde config.yaml. Respetar la clave `batch: maxAmount`, pero modificar el valor por defecto de modo tal que los paquetes no excedan los 8kB. 

Por su parte, el servidor deberá responder con éxito solamente si todas las apuestas del _batch_ fueron procesadas correctamente.

### Ejercicio N°7:

Modificar los clientes para que notifiquen al servidor al finalizar con el envío de todas las apuestas y así proceder con el sorteo.
Inmediatamente después de la notificacion, los clientes consultarán la lista de ganadores del sorteo correspondientes a su agencia.
Una vez el cliente obtenga los resultados, deberá imprimir por log: `action: consulta_ganadores | result: success | cant_ganadores: ${CANT}`.

El servidor deberá esperar la notificación de las 5 agencias para considerar que se realizó el sorteo e imprimir por log: `action: sorteo | result: success`.
Luego de este evento, podrá verificar cada apuesta con las funciones `load_bets(...)` y `has_won(...)` y retornar los DNI de los ganadores de la agencia en cuestión. Antes del sorteo no se podrán responder consultas por la lista de ganadores con información parcial.

Las funciones `load_bets(...)` y `has_won(...)` son provistas por la cátedra y no podrán ser modificadas por el alumno.

No es correcto realizar un broadcast de todos los ganadores hacia todas las agencias, se espera que se informen los DNIs ganadores que correspondan a cada una de ellas.

## Parte 3: Repaso de Concurrencia
En este ejercicio es importante considerar los mecanismos de sincronización a utilizar para el correcto funcionamiento de la persistencia.

### Ejercicio N°8:

Modificar el servidor para que permita aceptar conexiones y procesar mensajes en paralelo. En caso de que el alumno implemente el servidor en Python utilizando _multithreading_,  deberán tenerse en cuenta las [limitaciones propias del lenguaje](https://wiki.python.org/moin/GlobalInterpreterLock).

## Condiciones de Entrega
Se espera que los alumnos realicen un _fork_ del presente repositorio para el desarrollo de los ejercicios y que aprovechen el esqueleto provisto tanto (o tan poco) como consideren necesario.

Cada ejercicio deberá resolverse en una rama independiente con nombres siguiendo el formato `ej${Nro de ejercicio}`. Se permite agregar commits en cualquier órden, así como crear una rama a partir de otra, pero al momento de la entrega deberán existir 8 ramas llamadas: ej1, ej2, ..., ej7, ej8.
 (hint: verificar listado de ramas y últimos commits con `git ls-remote`)

Se espera que se redacte una sección del README en donde se indique cómo ejecutar cada ejercicio y se detallen los aspectos más importantes de la solución provista, como ser el protocolo de comunicación implementado (Parte 2) y los mecanismos de sincronización utilizados (Parte 3).

Se proveen [pruebas automáticas](https://github.com/7574-sistemas-distribuidos/tp0-tests) de caja negra. Se exige que la resolución de los ejercicios pase tales pruebas, o en su defecto que las discrepancias sean justificadas y discutidas con los docentes antes del día de la entrega. El incumplimiento de las pruebas es condición de desaprobación, pero su cumplimiento no es suficiente para la aprobación. Respetar las entradas de log planteadas en los ejercicios, pues son las que se chequean en cada uno de los tests.

La corrección personal tendrá en cuenta la calidad del código entregado y casos de error posibles, se manifiesten o no durante la ejecución del trabajo práctico. Se pide a los alumnos leer atentamente y **tener en cuenta** los criterios de corrección informados  [en el campus](https://campusgrado.fi.uba.ar/mod/page/view.php?id=73393).
