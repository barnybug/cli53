@export
Feature: export
  Scenario: I can export a domain
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain 'a A 127.0.0.1'"
    And I run "cli53 export $domain"
    Then the output contains "$domain"

  Scenario: I can export a domain --full
    Given I have a domain "$domain"
    When I run "cli53 rrcreate $domain 'www A 127.0.0.1'"
    When I run "cli53 rrcreate $domain 'alias 86400 AWS ALIAS A www $self false'"
    And I run "cli53 export --full $domain"
    Then the output contains "alias.$domain.	86400	AWS	ALIAS	A www.$domain. $self false"

  Scenario: I can export a domain to a file
      Given I have a domain "$domain"
      When I run "cli53 rrcreate $domain 'www A 127.0.0.1'"
      And I run "cli53 export --output /tmp/testcli53 $domain"
      Then the output file "/tmp/testcli53" contains "$domain"
