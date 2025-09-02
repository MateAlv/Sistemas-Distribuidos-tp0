import socket
import logging
from .utils import deserialize_bet, store_bets, BET_PARTS_COUNT

BET_ACCEPTED = "BET_ACCEPTED\n"
BET_REJECTED = "BET_REJECTED\n"

class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._running = True
        self.__client_socks = []

    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """

        while self._running:
            try:
                client_sock = self.__accept_new_connection()
                self.__client_socks.append(client_sock)
                self.__handle_client_connection(client_sock)
            except:
                self.__graceful_shutdown()

    def __handle_client_connection(self, client_sock):
        """
        Read lottery bet from client socket and store it
        """
        try:
            msg = self.__receive_complete_message(client_sock)
            addr = client_sock.getpeername()
            logging.info(f'action: receive_message | result: success | ip: {addr[0]} | msg: {msg}')
            
            try:
                bet = deserialize_bet(msg)
                
                # Store bet using the provided function
                store_bets([bet])
                
                logging.info(f'action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}')
                
                confirmation = BET_ACCEPTED

                self.__send_complete_message(client_sock, confirmation)
            except ValueError as parse_error:
                logging.error(f'action: deserialize_bet | result: fail | ip: {addr[0]} | error: {parse_error}')
                rejection = BET_REJECTED
                try:
                    self.__send_complete_message(client_sock, rejection)
                except:
                    pass 
                
        except OSError as e:
            logging.error(f"action: receive_message | result: fail | error: {e}")
        except Exception as e:
            logging.error(f"action: handle_client_connection | result: fail | error: {e}")
        finally:
            client_sock.close()

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """

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

    def __receive_complete_message(self, client_sock):
        """
        Receive complete message until newline delimiter.
        Fails if newline not found or connection issues.
        """
        buffer = b""
        while b'\n' not in buffer:
            try:
                chunk = client_sock.recv(1024)
                if not chunk:
                    raise OSError("Connection closed before complete message received")
                buffer += chunk
            except OSError:
                raise 
        
        message = buffer.split(b'\n')[0].decode('utf-8')
        return message

    def __send_complete_message(self, client_sock, message):
        """
        Send complete message, ensuring all bytes are sent.
        """
        data = message.encode('utf-8')
        total_sent = 0
        
        while total_sent < len(data):
            try:
                sent = client_sock.send(data[total_sent:])
                if sent == 0:
                    raise OSError("Socket connection broken")
                total_sent += sent
            except OSError:
                raise 