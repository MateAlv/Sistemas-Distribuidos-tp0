import socket
import logging


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
        Read message from a specific client socket and closes the socket

        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        try:
            # TODO: Modify the receive to avoid short-reads
            msg = client_sock.recv(1024).rstrip().decode('utf-8')
            addr = client_sock.getpeername()
            logging.info(f'action: receive_message | result: success | ip: {addr[0]} | msg: {msg}')
            # TODO: Modify the send to avoid short-writes
            client_sock.send("{}\n".format(msg).encode('utf-8'))
        except OSError as e:
            logging.error("action: receive_message | result: fail | error: {e}")
        finally:
            client_sock.close()

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