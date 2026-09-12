#!/bin/sh

# PROVIDE: pathfinder
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="pathfinder"
rcvar="${name}_enable"
desc="Pathfinder threat intelligence service"

load_rc_config "${name}"

: ${pathfinder_enable:=NO}

: ${pathfinder_executable:=/usr/local/sbin/pathfinder}
: ${pathfinder_config:=/usr/local/etc/pathfinder/pathfinder.conf}
: ${pathfinder_secret:=/usr/local/etc/pathfinder/secrets/postgresql-password}

: ${pathfinder_state_dir:=/var/db/pathfinder/state}
: ${pathfinder_artifacts_dir:=/var/db/pathfinder/artifacts}
: ${pathfinder_log_dir:=/var/log/pathfinder}

: ${pathfinder_run_user:=pathfinder}
: ${pathfinder_run_group:=pathfinder}

: ${pathfinder_run_dir:=/var/run/pathfinder}
: ${pathfinder_child_pidfile:=/var/run/pathfinder/pathfinder.pid}
: ${pathfinder_supervisor_pidfile:=/var/run/pathfinder/pathfinder-supervisor.pid}

: ${pathfinder_log_file:=/var/log/pathfinder/pathfinder.log}

command="/usr/sbin/daemon"
procname="/usr/sbin/daemon"
pidfile="${pathfinder_supervisor_pidfile}"

required_files="${pathfinder_config} ${pathfinder_secret}"

pathfinder_flags="-c -P ${pathfinder_supervisor_pidfile} -p ${pathfinder_child_pidfile} -u ${pathfinder_run_user} -o ${pathfinder_log_file} -t pathfinder"
command_args="${pathfinder_executable} serve -config ${pathfinder_config}"

start_precmd="pathfinder_prestart"
stop_postcmd="pathfinder_poststop"

extra_commands="validate"
validate_cmd="pathfinder_validate"

pathfinder_poststop()
{
    rm -f \
        "${pathfinder_child_pidfile}" \
        "${pathfinder_supervisor_pidfile}"
}

pathfinder_prestart()
{
    pathfinder_validate || return 1

    if [ ! -x "${pathfinder_executable}" ]; then
        warn "Pathfinder executable not installed: ${pathfinder_executable}"
        return 1
    fi

    install \
        -d \
        -o root \
        -g "${pathfinder_run_group}" \
        -m 0750 \
        "${pathfinder_run_dir}" || return 1

    if [ ! -e "${pathfinder_log_file}" ]; then
        install \
            -o "${pathfinder_run_user}" \
            -g "${pathfinder_run_group}" \
            -m 0640 \
            /dev/null \
            "${pathfinder_log_file}" || return 1
    else
        chown \
            "${pathfinder_run_user}:${pathfinder_run_group}" \
            "${pathfinder_log_file}" || return 1

        chmod 0640 "${pathfinder_log_file}" || return 1
    fi
}

pathfinder_validate()
{
    _failed=0

    echo "Pathfinder service contract validation"

    if id "${pathfinder_run_user}" >/dev/null 2>&1; then
        echo "  service identity:      PASS"
    else
        echo "  service identity:      FAIL"
        _failed=1
    fi

    if [ -r "${pathfinder_config}" ]; then
        echo "  configuration:         PASS"
    else
        echo "  configuration:         FAIL"
        _failed=1
    fi

    if [ -r "${pathfinder_secret}" ]; then
        echo "  database secret:       PASS"
    else
        echo "  database secret:       FAIL"
        _failed=1
    fi

    if su -m "${pathfinder_run_user}" -c \
        "test -r '${pathfinder_config}'"
    then
        echo "  service config read:   PASS"
    else
        echo "  service config read:   FAIL"
        _failed=1
    fi

    if su -m "${pathfinder_run_user}" -c \
        "test ! -w '${pathfinder_config}'"
    then
        echo "  config write denied:   PASS"
    else
        echo "  config write denied:   FAIL"
        _failed=1
    fi

    if su -m "${pathfinder_run_user}" -c \
        "test -r '${pathfinder_secret}'"
    then
        echo "  service secret read:   PASS"
    else
        echo "  service secret read:   FAIL"
        _failed=1
    fi

    for _dir in \
        "${pathfinder_state_dir}" \
        "${pathfinder_artifacts_dir}" \
        "${pathfinder_log_dir}"
    do
        if su -m "${pathfinder_run_user}" -c \
            "test -w '${_dir}'"
        then
            echo "  writable ${_dir}: PASS"
        else
            echo "  writable ${_dir}: FAIL"
            _failed=1
        fi
    done

    if [ -x "${pathfinder_executable}" ]; then
        echo "  executable:            INSTALLED"
    else
        echo "  executable:            PENDING"
    fi

    [ "${_failed}" -eq 0 ]
}

run_rc_command "$1"
