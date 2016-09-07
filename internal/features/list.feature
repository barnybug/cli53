@list
Feature: list
  Scenario: I can list domains
    Given I have a domain "$domain"
    When I run "cli53 list"
    Then the output contains "$domain"

  Scenario: I can list domains as csv
    Given I have a domain "$domain"
    When I run "cli53 list --format csv"
    Then the output contains "id,name,record count,comment"
    And the output contains "$domain.,2,"

  Scenario: I can list domains as json
    Given I have a domain "$domain"
    When I run "cli53 list --format json"
    Then the output contains "[{"
    And the output contains "$domain"
    And the output contains "}]"

  Scenario: I can list domains as jl
    Given I have a domain "$domain"
    When I run "cli53 list --format jl"
    Then the output contains "{"
    And the output contains "$domain"
    And the output contains "}"

  Scenario: I can list domains as text
    Given I have a domain "$domain"
    When I run "cli53 list --format text"
    Then the output contains "Name: \"$domain.\""

  Scenario: I can list domains as table
    Given I have a domain "$domain"
    When I run "cli53 list --format table"
    Then the output matches "ID +Name +Record count +Comment"
    And the output contains "$domain. 2"

  Scenario: list validates format parameter
    Given I have a domain "$domain"
    When I execute "cli53 list --format x"
    Then the exit code was 1
