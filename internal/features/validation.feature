@validation
Feature: parameter validation
  Scenario: identifier is required with failover
    When I execute "cli53 rrcreate --failover PRIMARY $domain 'a A 127.0.0.1'"
    Then the exit code was 1

  Scenario: identifier is required with weight
    When I execute "cli53 rrcreate --weight 10 $domain 'a A 127.0.0.1'"
    Then the exit code was 1

  Scenario: identifier is required with region
    When I execute "cli53 rrcreate --region us-west-1 $domain 'a A 127.0.0.1'"
    Then the exit code was 1

  Scenario: identifier alone is invalid
    When I execute "cli53 rrcreate -i id $domain 'a A 127.0.0.1'"
    Then the exit code was 1

  Scenario: failover must be PRIMARY/SECONDARY
    When I execute "cli53 rrcreate -i id --failover JUNK $domain 'a A 127.0.0.1'"
    Then the exit code was 1

  Scenario: failover and weight are mutually exclusive
    When I execute "cli53 rrcreate -i id --failover PRIMARY --weight 10 $domain 'a A 127.0.0.1'"
    Then the exit code was 1
