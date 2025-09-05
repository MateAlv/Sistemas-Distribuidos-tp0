import csv
import datetime
import logging

""" Bets storage location. """
STORAGE_FILEPATH = "./bets.csv"
""" Simulated winner number in the lottery contest. """
LOTTERY_WINNER_NUMBER = 7574
""" Number of parts in a bet. """
BET_PARTS_COUNT = 6  # AGENCY;NAME;LASTNAME;DOCUMENT;BIRTHDATE;NUMBER

# Protocol constants for batch processing
BATCH_FIELD_SEPARATOR = ";"
BATCH_SEPARATOR = "~"
BATCH_HEADER_PREFIX = "S:"


""" A lottery bet registry. """
class Bet:
    def __init__(self, agency: str, first_name: str, last_name: str, document: str, birthdate: str, number: str):
        """
        agency must be passed with integer format.
        birthdate must be passed with format: 'YYYY-MM-DD'.
        number must be passed with integer format.
        """
        self.agency = int(agency)
        self.first_name = first_name
        self.last_name = last_name
        self.document = document
        self.birthdate = datetime.date.fromisoformat(birthdate)
        self.number = int(number)

    def serialize(self) -> str:
        """
        Serialize bet to protocol format: AGENCY;NAME;LASTNAME;DOCUMENT;BIRTHDATE;NUMBER
        """
        return f"{self.agency}{BATCH_FIELD_SEPARATOR}{self.first_name}{BATCH_FIELD_SEPARATOR}{self.last_name}{BATCH_FIELD_SEPARATOR}{self.document}{BATCH_FIELD_SEPARATOR}{self.birthdate.isoformat()}{BATCH_FIELD_SEPARATOR}{self.number}"



""" Checks whether a bet won the prize or not. """
def has_won(bet: Bet) -> bool:
    return bet.number == LOTTERY_WINNER_NUMBER

"""
Persist the information of each bet in the STORAGE_FILEPATH file.
Not thread-safe/process-safe.
"""
def store_bets(bets: list[Bet]) -> None:
    with open(STORAGE_FILEPATH, 'a+') as file:
        logging.debug(f"action: fd_open | result: success | kind: file | fd:{file.fileno()} | path:{STORAGE_FILEPATH} | mode:a+")
        writer = csv.writer(file, quoting=csv.QUOTE_MINIMAL)
        for bet in bets:
            writer.writerow([bet.agency, bet.first_name, bet.last_name,
                             bet.document, bet.birthdate, bet.number])
    logging.debug(f"action: fd_close | result: success | kind: file | path:{STORAGE_FILEPATH}")

"""
Loads the information all the bets in the STORAGE_FILEPATH file.
Not thread-safe/process-safe.
"""
def load_bets() -> list[Bet]: # type: ignore
    with open(STORAGE_FILEPATH, 'r') as file:
        reader = csv.reader(file, quoting=csv.QUOTE_MINIMAL)
        for row in reader:
            yield Bet(row[0], row[1], row[2], row[3], row[4], row[5])

def load_winning_bets(agency: int) -> list[str]:
    """
    Loads only the winning bets DNIs for a specific agency from STORAGE_FILEPATH.
    Returns list of DNIs of winners for the given agency.
    Memory efficient - filters while reading, doesn't load all bets.
    Not thread-safe/process-safe.
    """
    winning_dnis = []
    
    try:
        with open(STORAGE_FILEPATH, 'r') as file:
            logging.debug(f"action: fd_open | result: success | kind: file | fd:{file.fileno()} | path:{STORAGE_FILEPATH} | mode:r")

            reader = csv.reader(file, quoting=csv.QUOTE_MINIMAL)
            for row in reader:
                if len(row) >= BET_PARTS_COUNT:
                    bet = Bet(row[0], row[1], row[2], row[3], row[4], row[5])
                    
                    # Filter: only this agency AND winning number is returned
                    if bet.agency == agency and has_won(bet):
                        winning_dnis.append(bet.document)
        logging.debug(f"action: fd_close | result: success | kind: file | path:{STORAGE_FILEPATH}")
        
    except FileNotFoundError:
        logging.warning(f"action: load_winning_bets | agency: {agency} | result: file_not_found")
        return []
    except Exception as e:
        logging.error(f"action: load_winning_bets | agency: {agency} | error: {e}")
        return []
    
    return winning_dnis


def deserialize_bet(bet_str: str) -> Bet:
    """
    Deserialize a bet from protocol format: AGENCY;NAME;LASTNAME;DOCUMENT;BIRTHDATE;NUMBER
    """
    parts = bet_str.split(BATCH_FIELD_SEPARATOR)
    if len(parts) != BET_PARTS_COUNT:
        raise ValueError(f"Invalid bet format: expected {BET_PARTS_COUNT} parts, got {len(parts)}")
    
    return Bet(parts[0].strip(), parts[1].strip(), parts[2].strip(), 
               parts[3].strip(), parts[4].strip(), parts[5].strip())


def deserialize_batch(message: str) -> list[Bet]:
    """
    Expect:
      S:<count>
      bet1~bet2~...~betN
    """
    lines = message.strip().split('\n')
    if len(lines) != 2:
        raise ValueError(f"Invalid batch format: expected 2 lines, got {len(lines)}")

    header = lines[0]
    if not header.startswith(BATCH_HEADER_PREFIX):
        raise ValueError(f"Invalid batch header: {header}")

    expected_count = int(header.split(":")[1])

    batch_data = lines[1]
    bets_str = batch_data.split(BATCH_SEPARATOR)

    if len(bets_str) != expected_count:
        raise ValueError(f"Batch size mismatch: expected {expected_count}, got {len(bets_str)}")

    bets = []
    for bet_str in bets_str:
        if bet_str.strip():
            bets.append(deserialize_bet(bet_str.strip()))
    return bets

def process_winning_bets(bets: list[Bet]) -> list[Bet]:
    """
    Filter and return only winning bets from a batch
    """
    winning_bets = []
    for bet in bets:
        if has_won(bet):
            winning_bets.append(bet)
    return winning_bets
