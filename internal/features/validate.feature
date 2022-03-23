@validate
Feature: validate a zone file syntax
  Scenario: correct zone file validation succeeds
    When I execute "cli53 validate --file tests/validate1.txt"
    Then the exit code was 0
  Scenario: incorrect zone file fails validation
    When I execute "cli53 validate --file tests/validate2.txt"
    Then the exit code was 1
