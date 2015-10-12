@commands
Feature: commands
  Scenario: I can create a resource record
    Given I have a domain "test.example.com"
    When I run "cli53 rrcreate test.example.com 'a A 127.0.0.1'"
    Then the domain "test.example.com" has record "a.test.example.com. 3600 IN A 127.0.0.1"

  Scenario: I can create a resource record (full)
    Given I have a domain "test.example.com"
    When I run "cli53 rrcreate test.example.com 'a.test.example.com. A 127.0.0.1'"
    Then the domain "test.example.com" has record "a.test.example.com. 3600 IN A 127.0.0.1"

  Scenario: I can create a failover record
    Given I have a domain "test.example.com"
    When I run "cli53 rrcreate -i "The First" --failover PRIMARY --health-check 6bb57c41-879a-42d0-acdd-ed6472f08eb9 test.example.com 'failover 300 IN A 127.0.0.1'"
    Then the domain "test.example.com" has record "failover.test.example.com. 300 IN A 127.0.0.1 ; AWS routing="FAILOVER" failover="PRIMARY" healthCheckId="6bb57c41-879a-42d0-acdd-ed6472f08eb9" identifier="The First""

  Scenario: I can create a geolocation record
    Given I have a domain "test.example.com"
    When I run "cli53 rrcreate -i Africa --continent-code AF test.example.com 'geo 300 IN A 127.0.0.1'"
    Then the domain "test.example.com" has record "geo.test.example.com. 300 IN A 127.0.0.1 ; AWS routing="GEOLOCATION" continentCode="AF" identifier="Africa""

  Scenario: I can create a latency record
    Given I have a domain "test.example.com"
    When I run "cli53 rrcreate -i USWest1 --region us-west-1 test.example.com 'latency 300 IN A 127.0.0.1'"
    Then the domain "test.example.com" has record "latency.test.example.com. 300 IN A 127.0.0.1 ; AWS routing="LATENCY" region="us-west-1" identifier="USWest1""

  Scenario: I can create a weighted record
    Given I have a domain "test.example.com"
    When I run "cli53 rrcreate -i One --weight 1 test.example.com 'weighted 300 IN A 127.0.0.1'"
    Then the domain "test.example.com" has record "weighted.test.example.com. 300 IN A 127.0.0.1 ; AWS routing="WEIGHTED" weight=1 identifier="One""

  Scenario: I can delete a resource record
    Given I have a domain "test.example.com"
    When I run "cli53 rrcreate test.example.com 'a A 127.0.0.1'"
    And I run "cli53 rrdelete test.example.com a A"
    Then the domain "test.example.com" doesn't have record "a.test.example.com. 3600 IN A 127.0.0.1"

  Scenario: I can delete a resource record by identifier
    Given I have a domain "test.example.com"
    When I run "cli53 rrcreate -i One --weight 1 test.example.com 'weighted.test.example.com. 300 IN A 127.0.0.1'"
    And I run "cli53 rrcreate -i Two --weight 2 test.example.com 'weighted.test.example.com. 300 IN A 127.0.0.2'"
    And I run "cli53 rrdelete -i One test.example.com weighted A"
    Then the domain "test.example.com" doesn't have record "weighted.test.example.com. 300 IN A 127.0.0.1 ; AWS routing="WEIGHTED" weight=1 identifier="One""
    And the domain "test.example.com" has record "weighted.test.example.com. 300 IN A 127.0.0.2 ; AWS routing="WEIGHTED" weight=2 identifier="Two""
