@bind
Feature: bind files
  Scenario: I can import a basic zone
    Given I have a domain "basic.example.com"
    When I run "cli53 import --file tests/basic.txt basic.example.com"
    Then the domain "basic.example.com" export matches file "tests/basic.txt"

  Scenario: I can import an arpa zone
    Given I have a domain "0.1.10.in-addr.arpa"
    When I run "cli53 import --file tests/arpa.txt 0.1.10.in-addr.arpa"
    Then the domain "0.1.10.in-addr.arpa" export matches file "tests/arpa.txt"

  Scenario: I can import a big zone
    Given I have a domain "big.example.com"
    When I run "cli53 import --file tests/big.txt big.example.com"
    Then the domain "big.example.com" export matches file "tests/big.txt"

  Scenario: I can import a zone with failover extensions
    Given I have a domain "failover.example.com"
    When I run "cli53 import --file tests/failover.txt failover.example.com"
    Then the domain "failover.example.com" export matches file "tests/failover.txt"

  Scenario: I can import a zone with geo extensions
    Given I have a domain "geo.example.com"
    When I run "cli53 import --file tests/geo.txt geo.example.com"
    Then the domain "geo.example.com" export matches file "tests/geo.txt"

  Scenario: I can import a zone with latency extensions
    Given I have a domain "latency.example.com"
    When I run "cli53 import --file tests/latency.txt latency.example.com"
    Then the domain "latency.example.com" export matches file "tests/latency.txt"

  Scenario: I can import a zone with weighted extensions
    Given I have a domain "weighted.example.com"
    When I run "cli53 import --file tests/weighted.txt weighted.example.com"
    Then the domain "weighted.example.com" export matches file "tests/weighted.txt"

  Scenario: I can import a zone with alias extensions
    Given I have a domain "alias.example.com"
    When I run "cli53 import --file tests/alias.txt alias.example.com"
    When I run "cli53 import --file tests/alias2.txt alias.example.com"
    Then the domain "alias.example.com" export matches file "tests/alias3.txt"

  Scenario: I can import (replace) a zone
    Given I have a domain "replace.example.com"
    When I run "cli53 import --file tests/replace1.txt replace.example.com"
    And I run "cli53 import --replace --file tests/replace2.txt replace.example.com"
    Then the domain "replace.example.com" export matches file "tests/replace2.txt"

  Scenario: I can import a zone editing auth
    Given I have a domain "auth.example.com"
    When I run "cli53 import --file tests/auth.txt --replace --editauth auth.example.com"
    Then the domain "auth.example.com" export matches file "tests/auth.txt" including auth
