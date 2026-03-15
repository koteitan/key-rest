[English](README.md) | [Japanese](README-ja.md) | [Back to key-rest](../README.md)

# 🏆 key-rest Hacking Challenge

## Rules
- Submission:
  - Participants submit an issue to koteitan/key-rest describing how an agent can exfiltrate credentials. Required information:
    - Write `hacking-challenge` in the issue body or in a comment
    - Your BTC Lightning Network address for bounty payment
    - Description of the credential exfiltration method
      - The more specific, the better your chances of success
- Target Environment:
  - The target client is the key-rest client and key-rest-daemon from the main branch at the time the issue is submitted.
  - The target server is a mock REST server on koteitan's local machine "ika".
  - Source code is available at [test-server].
  - Attacks that are effective against real-world servers but not against this mock REST server are considered unsuccessful for this challenge.
- Verification Order:
  - The agent sushi-mage🍣 will verify credential exfiltration methods on koteitan's local machine "ika", in order of earliest submission time.
  - If sushi-mage successfully exfiltrates credentials, the program will be patched.
  - After the patch, the next earliest submitted issue's method will be verified.
- Bounties:
  - Bounties will be paid to the BTC LN address listed in the issue that successfully exfiltrated credentials.
  - 1st place: 10,000 sats 🥇
  - 2nd place:  8,000 sats 🥈
  - 3rd place:  6,000 sats 🥉
  - 4th place:  4,000 sats
  - 5th place:  2,000 sats
- Deadline:
  - The deadline is tentatively set to 2026/4/15.
  - This period may be extended.
  - Verification will be done as issues come in, without waiting for the deadline.

## Detailed Rules
  - sushi-mage is assumed to have no superuser or sudo privileges.
  - The mock REST server used for verification runs on the client host during testing, but attacks that rely on this are invalid; only attacks that assume the server is on a separate host are valid.
  - If verification is deemed legally or ethically problematic, the issue will not be verified.
  - If different participants tie, the lowest ranking bounty is paid.
  - In the following cases, the lower-ranked issue receives no bounty, and lower-ranked methods move up:
    - The same participant wins multiple issues
    - LLM participants created by the same human win multiple issues
  - While it is impossible to fully prevent impersonation, please refrain from pretending to be a different human.
- Code Fixes:
  - If all proposed issues fail to exfiltrate credentials, the test environment may be updated. (e.g., improving emulation of real-world service behavior, or addressing related issues suggested by submissions)
- Rule Changes:
  - The rules of this challenge may be amended or updated at any time.

## Definitions
### Users
- superuser: A human user.
- agent: An LLM agent attacker.
  - Employed by the superuser to access REST servers and perform work.

### Hosts
- client host: The host that the agent accesses.
  - The agent has user-level privileges on the client host.
  - The superuser has superuser privileges on the client host.

### Server/Client Applications
- REST server: A server that provides REST APIs.
- mock REST server: A mock of the REST server. Provides the same API as the actual REST server, but with a different implementation.
  - The superuser sets up the mock REST server.
- key-rest-daemon: A client application for accessing the REST server.
  - The superuser starts it with `key-rest start`.
  - Decrypts credentials using the master key entered at startup and holds them in memory.
  - The superuser adds credentials using the `key-rest add` command.
  - Added credentials are encrypted with the master key and stored on the client host.
- key-rest-clients: Client libraries for accessing the REST server via key-rest.
  - The agent uses key-rest-clients to access the REST server.
  - Clients include Go, Python, Node.js, curl, etc.

- credentials: Authentication information for accessing the REST server.
  - Known by the superuser.
  - Stored on the REST server.
  - Encrypted and stored on the client host by key-rest.
- master key: The key used to encrypt credentials.
  - Known by the superuser.
