@create
Feature: create
  Scenario: I can create a domain
    When I run "cli53 create $domain --comment hi"
    Then the domain "$domain" is created

  Scenario: I can create a domain period
    When I run "cli53 create $domain. --comment hi"
    Then the domain "$domain" is created

  Scenario: I can create a VPC private domain
    When I run "cli53 create $domain --comment hi --vpc-id vpc-d70f05b5 --vpc-region eu-west-1"
    Then the domain "$domain" is created
