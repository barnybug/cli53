@commands
Feature: commands
  Scenario: I can create a resource record
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain 'a A 127.0.0.1'"
    Then the domain "$domain" has record "a.$domain. 3600 IN A 127.0.0.1"

  Scenario: I can create a resource record (full)
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain 'a.$domain. A 127.0.0.1'"
    Then the domain "$domain" has record "a.$domain. 3600 IN A 127.0.0.1"

  Scenario: I can create a failover record
    Given I have a domain "$domain"
    When I run "cli53 rrcreate -i "The First" --failover PRIMARY --health-check 6bb57c41-879a-42d0-acdd-ed6472f08eb9 $domain 'failover 300 IN A 127.0.0.1'"
    Then the domain "$domain" has record "failover.$domain. 300 IN A 127.0.0.1 ; AWS routing="FAILOVER" failover="PRIMARY" healthCheckId="6bb57c41-879a-42d0-acdd-ed6472f08eb9" identifier="The First""

  Scenario: I can create a geolocation record
    Given I have a domain "$domain"
    When I run "cli53 rrcreate -i Africa --continent-code AF $domain 'geo 300 IN A 127.0.0.1'"
    Then the domain "$domain" has record "geo.$domain. 300 IN A 127.0.0.1 ; AWS routing="GEOLOCATION" continentCode="AF" identifier="Africa""

  Scenario: I can create a latency record
    Given I have a domain "$domain"
    When I run "cli53 rrcreate -i USWest1 --region us-west-1 $domain 'latency 300 IN A 127.0.0.1'"
    Then the domain "$domain" has record "latency.$domain. 300 IN A 127.0.0.1 ; AWS routing="LATENCY" region="us-west-1" identifier="USWest1""

  Scenario: I can create a weighted record
    Given I have a domain "$domain"
    When I run "cli53 rrcreate -i One --weight 1 $domain 'weighted 300 IN A 127.0.0.1'"
    Then the domain "$domain" has record "weighted.$domain. 300 IN A 127.0.0.1 ; AWS routing="WEIGHTED" weight=1 identifier="One""

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
