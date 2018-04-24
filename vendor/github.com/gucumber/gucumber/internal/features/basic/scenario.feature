@scenario @fast
Feature: Scenarios
  This package should support the use of scenarios.

  @basic
  Scenario: Basic usage
    Given I have an initial step
    And I have a second step
    When I run the "gucumber" command
    Then this scenario should execute 1 time and pass
    And setup was called 1 time

  @outline
  Scenario Outline: Scenario outline
    Given I perform <val1> + <val2>
    Then I should get <result>
    And setup was called 1 time

    Examples:
      | val1 | val2 | result |
      | 1    | 2    | 3      |
      | 3    | 4    | 7      |
