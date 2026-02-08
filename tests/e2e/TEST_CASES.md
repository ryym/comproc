# E2E Test Cases

Comprehensive test cases for verifying comproc's command behavior.
Focus: process start/stop correctness, command options, dependency resolution, restart policies.

## 1. up

| #    | Test                              | Description                                                                  |
| ---- | --------------------------------- | ---------------------------------------------------------------------------- |
| 1.1  | TestUp_SingleService              | Start a single service; verify state=running and PID is assigned             |
| 1.2  | TestUp_MultipleServices           | Start multiple services at once; all become running                          |
| 1.3  | TestUp_SpecificServices           | `up svc1 svc2` starts only specified services; others remain stopped         |
| 1.4  | TestUp_SpecificServiceWithDeps    | `up api` auto-starts its dependency (db) as well                             |
| 1.5  | TestUp_AlreadyRunning             | Running `up` again while daemon is active does not disrupt existing services |
| 1.6  | TestUp_StartStoppedService        | After `stop svc`, `up svc` restarts it                                       |
| 1.7  | TestUp_FollowLogs                 | `up -f` streams logs; Ctrl-C disconnects but daemon keeps running            |
| 1.8  | TestUp_FollowLogsSpecificServices | `up -f svc1` starts only svc1 and follows its logs                           |
| 1.9  | TestUp_StartsOnlyNewServices      | While daemon runs, `up newSvc` starts only the not-yet-running service       |
| 1.10 | TestUp_MultipleServicesWithDeps   | `up` starts all services respecting dependency order (db→api→frontend)       |

## 2. down

| #   | Test                             | Description                                                       |
| --- | -------------------------------- | ----------------------------------------------------------------- |
| 2.1 | TestDown_StopsAllAndShutsDaemon  | Stops all services and shuts down daemon (socket removed)         |
| 2.2 | TestDown_MultipleRunningServices | All running services appear in the stopped list                   |
| 2.3 | TestDown_NoDaemon                | Succeeds silently when no daemon is running                       |
| 2.4 | TestDown_WithDependencies        | Services with dependencies are stopped in correct (reverse) order |

## 3. stop

| #   | Test                        | Description                                                      |
| --- | --------------------------- | ---------------------------------------------------------------- |
| 3.1 | TestStop_SpecificService    | Stops only the specified service; others remain running          |
| 3.2 | TestStop_DaemonStaysRunning | Daemon stays alive after stopping services (socket still exists) |
| 3.3 | TestStop_AllServices        | `stop` with no args stops all services; daemon stays alive       |
| 3.4 | TestStop_StopsDependents    | Stopping db also stops api that depends on it                    |
| 3.5 | TestStop_MultipleServices   | `stop svc1 svc2` stops multiple specified services               |
| 3.6 | TestStop_AlreadyStopped     | Stopping an already-stopped service succeeds with no error       |
| 3.7 | TestStop_NoDaemon           | Succeeds with no error when no daemon is running (same as 3.6)   |

## 4. restart

| #   | Test                         | Description                                                    |
| --- | ---------------------------- | -------------------------------------------------------------- |
| 4.1 | TestRestart_SingleService    | PID changes after restart; state returns to running            |
| 4.2 | TestRestart_AllServices      | `restart` with no args restarts all services                   |
| 4.3 | TestRestart_MultipleSpecific | `restart svc1 svc2` restarts only specified services           |
| 4.4 | TestRestart_AlreadyStopped   | Restarting a stopped service starts it (equivalent to `up`)    |
| 4.5 | TestRestart_NoDaemon         | Succeeds with no error when no daemon is running (same as 4.4) |

## 5. status / ps

| #   | Test                          | Description                                                       |
| --- | ----------------------------- | ----------------------------------------------------------------- |
| 5.1 | TestStatus_RunningServices    | Shows correct NAME, STATE=running, PID, RESTARTS for live service |
| 5.2 | TestStatus_AfterStop          | Stopped service shows STATE=stopped, PID="-"                      |
| 5.3 | TestStatus_PsAlias            | `ps` produces the same output as `status`                         |
| 5.4 | TestStatus_NoDaemonWithConfig | Without daemon but with config, all services shown as stopped     |
| 5.5 | TestStatus_NoDaemonNoConfig   | Without daemon or config, prints "No services defined"            |
| 5.6 | TestStatus_NormalExit         | Process exits with 0 (restart:never) -> state=stopped             |
| 5.7 | TestStatus_FailedExit         | Process exits with 1 (restart:never) -> state=failed              |

## 6. logs

| #   | Test                   | Description                                            |
| --- | ---------------------- | ------------------------------------------------------ |
| 6.1 | TestLogs_RecentLines   | Retrieves recent log lines from a running service      |
| 6.2 | TestLogs_ServiceFilter | Filters logs to show only the specified service        |
| 6.3 | TestLogs_LineLimit     | `-n 5` limits the number of returned lines             |
| 6.4 | TestLogs_NoDaemon      | Returns empty output without error when no daemon runs |
| 6.5 | TestLogs_FollowMode    | `logs -f` streams new log lines in real time           |

## 7. Restart Policies

| #   | Test                                    | Description                                                 |
| --- | --------------------------------------- | ----------------------------------------------------------- |
| 7.1 | TestRestartPolicy_Never                 | Process exits with 0; not restarted, restarts=0             |
| 7.2 | TestRestartPolicy_OnFailure_NonZeroExit | Process exits with 1; restarted (restarts >= 1)             |
| 7.3 | TestRestartPolicy_OnFailure_ZeroExit    | Process exits with 0; not restarted under on-failure policy |
| 7.4 | TestRestartPolicy_Always                | Process exits with 0; still restarted under always policy   |
| 7.5 | TestRestartPolicy_CounterIncrements     | Restarts counter increases with each restart                |

## 8. Config: env / working_dir

| #   | Test                  | Description                                                 |
| --- | --------------------- | ----------------------------------------------------------- |
| 8.1 | TestConfig_EnvVars    | Environment variables from config are passed to the process |
| 8.2 | TestConfig_WorkingDir | working_dir is used as the process's working directory      |
