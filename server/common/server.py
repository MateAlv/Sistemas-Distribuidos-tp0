import os
import socket
import logging
import threading
from .utils import (deserialize_batch, store_bets, load_winning_bets, BATCH_SEPARATOR)

# File lock for thread-safe file operations
_FILE_LOCK = threading.Lock()

# Message protocol constants
MESSAGE_DELIMITER = b"\n"
SUCCESS_RESPONSE = "OK"
FAILURE_RESPONSE = "FAIL"
FINISHED_MESSAGE = "FINISHED"
WINNERS_PREFIX = "WINNERS:"
NO_WINNERS_RESPONSE = "N"

class Server:
    def __init__(self, port, listen_backlog):
        self.port = port
        self.listen_backlog = listen_backlog
        
        # Socket setup
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        
        # Server state
        self._running = True
        
        # Thread management
        self.client_threads = []
        
        # Get expected agencies from environment
        expected_agencies = int(os.getenv('CLI_CLIENTS', '5'))

        # Barrier and Locks for lottery synchronization
        self.lottery_barrier = threading.Barrier(expected_agencies)
        
        logging.info(f"action: server_init | result: success | expected_agencies: {expected_agencies}")
        

    def run(self):
        """Server loop - accept connections and spawn threads"""
        logging.info("action: accept_connections | result: in_progress")
        
        try:
            while self._running:
                try:
                    client_sock, addr = self._server_socket.accept()
                    if not client_sock:
                        logging.error("action: accept_connection | result: fail | reason: null_socket")
                        continue
                    logging.debug(f"action: accept_connection | result: success | ip: {addr[0]}")
                    
                    # Create thread to handle client
                    client_thread = threading.Thread(
                        target=self.__handle_client_connection,
                        args=(client_sock,)
                    )
                    client_thread.daemon = False  # Must finish properly
                    client_thread.start()
                    
                    # Track active threads
                    self.client_threads.append(client_thread)
                    
                    # Cleanup finished threads
                    self.__cleanup_finished_threads()
                    
                except OSError:
                    # Socket closed during shutdown
                    if self._running:
                        logging.error("action: accept_connection | result: fail")
                    break
                    
        except Exception as e:
            logging.error(f"action: server_loop | result: fail | error: {e}")
        finally:
            self.__graceful_shutdown()

    def __cleanup_finished_threads(self):
        """Remove finished threads from tracking list"""
        self.client_threads = [t for t in self.client_threads if t.is_alive()]
        
    def __handle_client_connection(self, client_sock):
        """Handle communication with a connected client"""
        client_agency = None
        
        try:
            while True:
                msg = self.__receive_complete_message(client_sock)
                if not msg:
                    break
                    
                if msg.startswith("S:"):
                    bets = deserialize_batch(msg)
                    if bets:
                        client_agency = bets[0].agency
                        with _FILE_LOCK:
                            store_bets(bets)
                        logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
                    self.__send_complete_message(client_sock, SUCCESS_RESPONSE)
                    
                elif msg == "FINISHED":
                    # Wait for all agencies, then return winners
                    self.__handle_finished_and_return_winners(client_sock, client_agency)
                    break  # Client disconnects after getting winners
                    
                else:
                    logging.warning(f"action: unknown_message | message: {msg}")
                    self.__send_complete_message(client_sock, FAILURE_RESPONSE)
                    
        except Exception as e:
            logging.error(f"action: client_handler_error | error: {e}")
        finally:
            try:
                client_sock.close()
            except:
                pass

    def __handle_finished_and_return_winners(self, client_sock, client_agency):
        """Handle FINISHED message - wait for lottery and return winners"""
        winning_dnis = [] 
        try:
            try:
                # Wait for all agencies to reach this point
                index = self.lottery_barrier.wait(timeout=120)
                if index == 0:  # First past barrier logs lottery
                    logging.info("action: sorteo | result: success")
            except Exception as e:
                logging.error(f"action: lottery_barrier_error | error: {e}")

            # Lottery is complete - load and return winners for this agency - THREAD SAFETY FIRST
            with _FILE_LOCK:
                winning_dnis = load_winning_bets(client_agency)

            if winning_dnis:
                response = WINNERS_PREFIX + BATCH_SEPARATOR.join(winning_dnis)
            else:
                response = NO_WINNERS_RESPONSE  # Indicate no winners for this agency

            self.__send_complete_message(client_sock, response)
            logging.info(
                f"action: winners_sent | result: success | agency: {client_agency} | cant_ganadores: {len(winning_dnis)}"
            )

        except Exception as e:
            logging.error(f"action: client_handler_error | error: {e}")
            try:
                self.__send_complete_message(client_sock, FAILURE_RESPONSE)
            except:
                pass

    def __receive_complete_message(self, client_sock):
        """
        Receive complete batch message until final delimiter.
        Handles multi-line messages (BATCH_SIZE:N\nbet1~bet2~bet3\n)
        """
        buffer = b""
        lines_received = 0
        expected_lines = 2  # Header + batch data
        
        while lines_received < expected_lines:
            try:
                chunk = client_sock.recv(1024)
                if not chunk:
                    raise OSError("Connection closed before complete message received")
                buffer += chunk
                
                lines_received = buffer.count(MESSAGE_DELIMITER)
                
            except OSError:
                raise 
        
        message = buffer.rstrip(b'\n').decode('utf-8')
        return message

    def __send_complete_message(self, client_sock, message):
        """
        Send complete message, ensuring all bytes are sent.
        """
        full_message = message + MESSAGE_DELIMITER.decode('utf-8')  # "WINNERS:dni1~dni2\n"
        data = full_message.encode('utf-8')
        total_sent = 0
        
        while total_sent < len(data):
            sent = client_sock.send(data[total_sent:])
            if sent == 0:
                raise OSError("Socket connection broken")
            total_sent += sent

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