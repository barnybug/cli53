@validate
Feature: validate a zone file syntax
  Scenario: list validates format parameter
    When I execute "cli53 validate --file tests/validate1.txt"
    Then the exit code was 0
  Scenario: list validates format parameter
    When I execute "cli53 validate --file tests/validate2.txt"
    Then the exit code was 1
