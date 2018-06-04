@import
Feature: import
  Scenario: I can import a wildcard zone
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/wildcard.txt $domain"
    Then the domain "$domain" export matches file "tests/wildcard.txt"

  Scenario: I can import a basic zone
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/basic.txt $domain"
    Then the domain "$domain" export matches file "tests/basic.txt"

  Scenario: I can import an arpa zone
    Given I have a domain "0.1.10.in-addr.arpa"
    When I run "cli53 import --file tests/arpa.txt 0.1.10.in-addr.arpa"
    Then the domain "0.1.10.in-addr.arpa" export matches file "tests/arpa.txt"

  Scenario: I can import a big zone
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/big.txt $domain"
    Then the domain "$domain" export matches file "tests/big.txt"

  Scenario: I can import a big zone with identifiers
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/big2.txt $domain"
    Then the domain "$domain" export matches file "tests/big2.txt"

  Scenario: I can import a zone with failover extensions
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/failover.txt $domain"
    Then the domain "$domain" export matches file "tests/failover.txt"

  Scenario: I can import a zone with geo extensions
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/geo.txt $domain"
    Then the domain "$domain" export matches file "tests/geo.txt"

  # Scenario: I can import a zone with geo ALIAS records 
  #   Given I have a domain "$domain"
  #   When I run "cli53 import --file tests/geo_alias.txt $domain"
  #   Then the domain "$domain" export matches file "tests/geo_alias.txt"

  Scenario: I can import a zone with latency extensions
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/latency.txt $domain"
    Then the domain "$domain" export matches file "tests/latency.txt"

  Scenario: I can import a zone with weighted extensions
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/weighted.txt $domain"
    Then the domain "$domain" export matches file "tests/weighted.txt"

  @multivalue
  Scenario: I can import a zone with multivalue answer extensions
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/multivalue.txt $domain"
    Then the domain "$domain" export matches file "tests/multivalue.txt"

  Scenario: I can import a zone with alias extensions
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/alias.txt $domain"
    Then the domain "$domain" export matches file "tests/alias.txt"

  Scenario: I can import a zone with an alias with multiple types
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/alias_multiple_types.txt $domain"
    Then the domain "$domain" export matches file "tests/alias_multiple_types.txt"

  Scenario: I can import (replace) a zone
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/replace1.txt $domain"
    And I run "cli53 import --replace --file tests/replace2.txt $domain"
    Then the domain "$domain" export matches file "tests/replace2.txt"

  Scenario: I can import dry-run (with changes)
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/replace1.txt $domain"
    And I run "cli53 import --replace --file tests/replace2.txt --dry-run $domain"
    Then the output contains "Dry-run"
    And the output contains "+ mail.$domain.	86400	IN	A	10.0.0.4"
    And the output contains "- mail.$domain.	86400	IN	A	10.0.0.2"

  Scenario: I can import dry-run (no changes)
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/replace1.txt $domain"
    And I run "cli53 import --replace --file tests/replace1.txt --dry-run $domain"
    Then the output contains "no changes would have been made"

  Scenario: I can import a zone editing auth
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/auth.txt --replace --editauth $domain"
    Then the domain "$domain" export matches file "tests/auth.txt" including auth

  Scenario: I can import a zone with no changes
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/replace1.txt $domain"
    And I run "cli53 import --replace --file tests/replace1.txt $domain"
    Then the output contains "0 changes"

  Scenario: I can import a zone with a wildcard record with no changes
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/replace3.txt $domain"
    And I run "cli53 import --replace --file tests/replace3.txt $domain"
    Then the output contains "0 changes"

  Scenario: I can import a zone as case-insensitive
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/uppercase.txt $domain"
    And I run "cli53 import --replace --file tests/uppercase.txt $domain"
    Then the output contains "0 changes"
