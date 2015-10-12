@domain
Feature: domain management
  Scenario: I can create a domain
    When I run "cli53 create test.example.com --comment hi"
    Then the domain "test.example.com" is created

  Scenario: I can delete a domain by name
    Given I have a domain "test.example.com"
    When I run "cli53 delete test.example.com"
    Then the domain "test.example.com" is deleted

  Scenario: I can delete and purge a big domain
    Given I have a domain "big.example.com"
    When I run "cli53 import --file tests/big.txt big.example.com"
    And I run "cli53 delete --purge big.example.com"
    Then the domain "test.example.com" is deleted

  Scenario: I can list domains
    Given I have a domain "test.example.com"
    When I run "cli53 list"
    Then the output contains "test.example.com"
