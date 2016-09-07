@rrpurge
Feature: rrpurge
  Scenario: I can purge a domain
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain 'a A 127.0.0.1'"
    And I run "cli53 rrpurge --confirm $domain"
    Then the domain "$domain" doesn't have record "a.$domain. 3600 IN A 127.0.0.1"

  Scenario: I can purge a domain with wildcard records
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain '*.wildcard A 127.0.0.1'"
    And I run "cli53 rrpurge --confirm $domain"
    Then the domain "$domain" doesn't have record "*.wildcard.$domain. 3600 IN A 127.0.0.1"

  Scenario: I can purge a domain with child NS record
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain 'a NS b.example.com.'"
    And I run "cli53 rrpurge --confirm $domain"
    Then the domain "$domain" doesn't have record "a.$domain. 3600 IN NS b.example.com."
