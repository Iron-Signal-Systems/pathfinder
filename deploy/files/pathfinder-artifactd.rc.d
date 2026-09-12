#!/bin/sh

# PROVIDE: pathfinder_artifactd
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="pathfinder_artifactd"
rcvar="${name}_enable"
desc="Pathfinder exact-byte artifact preservation service"

load_rc_config "${name}"

: ${pathfinder_artifactd_enable:=NO}

: ${pathfinder_artifactd_executable:=/usr/local/sbin/pathfinder-artifactd}
: ${pathfinder_artifactd_objects_dir:=/var/db/pathfinder/artifacts/objects}
: ${pathfinder_artifactd_socket:=/var/run/pathfinder-artifact/preserve.sock}
: ${pathfinder_artifactd_max_bytes:=268435456}

: ${pathfinder_artifactd_run_user:=pfartifact}
: ${pathfinder_artifactd_run_group:=pfartifact}

: ${pathfinder_artifactd_run_dir:=/var/run/pathfinder-artifact}
: ${pathfinder_artifactd_child_pidfile:=/var/run/pathfinder-artifact/pathfinder-artifactd.pid}
: ${pathfinder_artifactd_supervisor_pidfile:=/var/run/pathfinder-artifact/pathfinder-artifactd-supervisor.pid}

: ${pathfinder_artifactd_log_dir:=/var/log/pathfinder-artifact}
: ${pathfinder_artifactd_log_file:=/var/log/pathfinder-artifact/pathfinder-artifactd.log}

command="/usr/sbin/daemon"
procname="/usr/sbin/daemon"
pidfile="${pathfinder_artifactd_supervisor_pidfile}"

pathfinder_artifactd_flags="-c -P ${pathfinder_artifactd_supervisor_pidfile} -p ${pathfinder_artifactd_child_pidfile} -u ${pathfinder_artifactd_run_user} -o ${pathfinder_artifactd_log_file} -t pathfinder-artifactd"
command_args="${pathfinder_artifactd_executable} -objects ${pathfinder_artifactd_objects_dir} -socket ${pathfinder_artifactd_socket} -max-bytes ${pathfinder_artifactd_max_bytes}"

start_precmd="pathfinder_artifactd_prestart"
stop_postcmd="pathfinder_artifactd_poststop"

extra_commands="validate"
validate_cmd="pathfinder_artifactd_validate"

pathfinder_artifactd_poststop()
{
    rm -f \
        "${pathfinder_artifactd_socket}" \
        "${pathfinder_artifactd_child_pidfile}" \
        "${pathfinder_artifactd_supervisor_pidfile}"
}

pathfinder_artifactd_prestart()
{
    pathfinder_artifactd_validate || return 1

    if [ ! -x "${pathfinder_artifactd_executable}" ]; then
        warn "Pathfinder artifact preservation executable not installed: ${pathfinder_artifactd_executable}"
        return 1
    fi

    install \
        -d \
        -o "${pathfinder_artifactd_run_user}" \
        -g "${pathfinder_artifactd_run_group}" \
        -m 0750 \
        "${pathfinder_artifactd_run_dir}" || return 1

    install \
        -d \
        -o "${pathfinder_artifactd_run_user}" \
        -g "${pathfinder_artifactd_run_group}" \
        -m 0750 \
        "${pathfinder_artifactd_log_dir}" || return 1

    if [ ! -e "${pathfinder_artifactd_log_file}" ]; then
        install \
            -o "${pathfinder_artifactd_run_user}" \
            -g "${pathfinder_artifactd_run_group}" \
            -m 0640 \
            /dev/null \
            "${pathfinder_artifactd_log_file}" || return 1
    else
        chown \
            "${pathfinder_artifactd_run_user}:${pathfinder_artifactd_run_group}" \
            "${pathfinder_artifactd_log_file}" || return 1

        chmod 0640 "${pathfinder_artifactd_log_file}" || return 1
    fi
}

pathfinder_artifactd_validate()
{
    _failed=0

    echo "Pathfinder artifact preservation service contract validation"

    if id "${pathfinder_artifactd_run_user}" >/dev/null 2>&1; then
        echo "  preservation identity: PASS"
    else
        echo "  preservation identity: FAIL"
        _failed=1
    fi

    if su -m "${pathfinder_artifactd_run_user}" -c \
        "test -w '${pathfinder_artifactd_objects_dir}'"
    then
        echo "  preservation object write: PASS"
    else
        echo "  preservation object write: FAIL"
        _failed=1
    fi

    if su -m pathfinder -c \
        "test -r '${pathfinder_artifactd_objects_dir}'"
    then
        echo "  runtime object read: PASS"
    else
        echo "  runtime object read: FAIL"
        _failed=1
    fi

    if su -m pathfinder -c \
        "test ! -w '${pathfinder_artifactd_objects_dir}'"
    then
        echo "  runtime object write denied: PASS"
    else
        echo "  runtime object write denied: FAIL"
        _failed=1
    fi

    case "${pathfinder_artifactd_max_bytes}" in
        ''|*[!0-9]*)
            echo "  artifact byte bound: FAIL"
            _failed=1
            ;;
        0)
            echo "  artifact byte bound: FAIL"
            _failed=1
            ;;
        *)
            echo "  artifact byte bound: PASS"
            ;;
    esac

    if [ -x "${pathfinder_artifactd_executable}" ]; then
        echo "  executable: INSTALLED"
    else
        echo "  executable: PENDING"
    fi

    [ "${_failed}" -eq 0 ]
}

run_rc_command "$1"
