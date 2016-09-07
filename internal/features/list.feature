@list
Feature: list
  Scenario: I can list domains
    Given I have a domain "$domain"
    When I run "cli53 list"
    Then the output contains "$domain"
