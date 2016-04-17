@delegationSets
Feature: reusable delegation sets
  Scenario: I can create with a delegation set
    Given I have a delegation set
    When I run "cli53 create $domain --delegation-set-id $delegationSet"
    Then the domain "$domain" is created

  Scenario: I can create a delegation set
    When I run "cli53 dscreate"
    Then the output matches "Created reusable delegation set ID: '(.+)'"
    And the delegation set "$1" is created

  Scenario: I can delete a delegation set
    Given I have a delegation set
    When I run "cli53 dsdelete $delegationSet"
    Then the delegation set "$delegationSet" is deleted

  Scenario: I can list delegation sets when there none
    When I run "cli53 dslist"
    Then the output contains "none"

  Scenario: I can list delegation sets with one
    Given I have a delegation set
    When I run "cli53 dslist"
    Then the output contains "- ID: /delegationset/"
