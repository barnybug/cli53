@parse
Feature: parse
  Scenario: list validates format parameter
    When I execute "cli53 parse --file tests/parse1.txt"
    Then the exit code was 0
  Scenario: list validates format parameter
    When I execute "cli53 parse --file tests/parse2.txt"
    Then the exit code was 1
