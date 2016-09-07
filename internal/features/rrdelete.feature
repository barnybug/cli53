@rrdelete
Feature: rrdelete
  Scenario: I can delete a resource record
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain 'a A 127.0.0.1'"
    And I run "cli53 rrdelete $domain a A"
    Then the domain "$domain" doesn't have record "a.$domain. 3600 IN A 127.0.0.1"

  Scenario: I can delete a resource record by identifier
    Given I have a domain "$domain"
    When I run "cli53 rrcreate -i One --weight 1 $domain 'weighted.$domain. 300 IN A 127.0.0.1'"
    And I run "cli53 rrcreate -i Two --weight 2 $domain 'weighted.$domain. 300 IN A 127.0.0.2'"
    And I run "cli53 rrdelete -i One $domain weighted A"
    Then the domain "$domain" doesn't have record "weighted.$domain. 300 IN A 127.0.0.1 ; AWS routing="WEIGHTED" weight=1 identifier="One""
    And the domain "$domain" has record "weighted.$domain. 300 IN A 127.0.0.2 ; AWS routing="WEIGHTED" weight=2 identifier="Two""

  Scenario: I can delete a wildcard record
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain '*.wildcard A 127.0.0.1'"
    And I run "cli53 rrdelete $domain *.wildcard A"
    Then the domain "$domain" doesn't have record "*.wildcard.$domain. 3600 IN A 127.0.0.1"
