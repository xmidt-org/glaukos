# Glaukos time-elapsed parser flow:
1. **Event type check**: Whenever glaukos gets an event, check that it is a `fully-manageable` event. If not, do not continue
2. **Get relevant events**: Get the history of events from codex and run through the list to find events related to the last reboot-cycle (all events with the second most recent boot-time up to events with a birthdate less than the incoming `fully-manageable` event). While doing this, also perform the following Comparator checks:
    * Make sure that the boot-time of the fully-manageable event is the newest boot-time. If it isn’t, add the `NewerBootTimeFound` tag to metrics and do not continue.
3. Run through each event in the list of relevant events (adding the incoming `fully-manageable` event to the list). For the entire list, perform the following checks. 
    * Checks that will result in cycle-tags (tags that are applied to an entire boot-cycle). These tags will be added to a counter:
        * Specific metadata fields are the same.
        * Transaction_uuid between events in this list are different.
        * An online and offline event exists for each session id in this list.
        * Among events with the same boot-time: the `metadata/fw-name`, `metadata/hw-last-reboot-reason`, `metadata/webpa-protocol` are all the same.
    * Checks that are run on each event in the list (providing tags for each individual event. Add all tags to a counter with the event type and reason as labels):
        * Event-type matches one of the possible outcomes.
        * Boot-time is present and within the configured time frame.
        * Birthdate is present and within the configured time frame.
        * Boot-time is present and after the same date in 2015.
        * All device-id occurrences within the event are consistent.
        * All time values in the destination are at least 10s after the boot-time.
        * Timestamps in the destination are within 60s of the birthdate.
4. If there are no cycle-tags:
    * Figure out if a cycle is simple or complex.
    * Find the reboot-pending event (if it exists) and calculate the time elapsed. If reboot-pending event doesn’t exist, subtract the birthdate of the `fully-manageable` event from the boot-time and calculate the time elapsed, adding to the proper histogram.