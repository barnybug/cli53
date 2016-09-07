@delete
Feature: delete
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

  Scenario: I can delete a domain with a child NS record
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain 'a NS b.example.com.'"
    And I run "cli53 delete --purge $domain"
    Then the domain "$domain" is deleted
