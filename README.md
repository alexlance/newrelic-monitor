newrelic-monitor
================

New Relic recently retired the Servers component of their monitoring product.
Presumably to encourage adoption of their Infrastructure section.

This small binary can be executed and will parse a `/etc/newrelic/nrsysmond.cfg`
file for a New Relic `license_key` field.

It will check system metrics like CPU, Disk, Memory and Swap and report them
back to New Relic once per minute.

This New Relic plugin can be discovered by going to the Plugins menu in New Relic
and adding charts or alerts etc for the following components:

    Component/CPU[percent]
    Component/Disk[percent]
    Component/Memory[percent]
    Component/Swap[percent]
