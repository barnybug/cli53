@domain
Feature: domain management
  Scenario: I can create a domain
    When I run "cli53 create $domain --comment hi"
    Then the domain "$domain" is created

  Scenario: I can create a domain period
    When I run "cli53 create $domain. --comment hi"
    Then the domain "$domain" is created

  Scenario: I can delete a domain by name
    Given I have a domain "$domain"
    When I run "cli53 delete $domain"
    Then the domain "$domain" is deleted

  Scenario: I can delete a domain by name period
    Given I have a domain "$domain"
    When I run "cli53 delete $domain."
    Then the domain "$domain" is deleted

  Scenario: I can delete and purge a big domain
    Given I have a domain "$domain"
    When I run "cli53 import --file tests/big.txt $domain"
    And I run "cli53 delete --purge $domain"
    Then the domain "$domain" is deleted

  Scenario: I can list domains
    Given I have a domain "$domain"
    When I run "cli53 list"
    Then the output contains "$domain"

  Scenario: I can purge a domain
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain 'a A 127.0.0.1'"
    And I run "cli53 rrpurge --confirm $domain"
    Then the domain "$domain" doesn't have record "a.$domain. 3600 IN A 127.0.0.1"

  Scenario: I can export a domain
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain 'a A 127.0.0.1'"
    And I run "cli53 export $domain"
    Then the output contains "$domain"
