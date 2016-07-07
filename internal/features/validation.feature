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

  Scenario: passing --append and --replace at the same time makes no sense
    When I execute "cli53 rrcreate --append --replace $domain 'a A 127.0.0.2'"
    Then the exit code was 1

  Scenario: create requires one argument
    When I execute "cli53 create a b"
    Then the exit code was 1

  Scenario: delete requires one argument
    When I execute "cli53 delete a b"
    Then the exit code was 1

  Scenario: import requires one argument
    When I execute "cli53 import a b"
    Then the exit code was 1

  Scenario: export requires one argument
    When I execute "cli53 export a b"
    Then the exit code was 1

  Scenario: rrcreate requires two arguments
    When I execute "cli53 import a b c"
    Then the exit code was 1

  Scenario: rrdelete requires three arguments
    When I execute "cli53 import a b c d"
    Then the exit code was 1

  Scenario: rrpurge requires one argument
    When I execute "cli53 rrpurge a b"
    Then the exit code was 1

  Scenario: bad usage
    When I execute "cli53 --bad list"
    Then the exit code was 1
