# frozen_string_literal: true
# SPDX-License-Identifier: Apache-2.0

# Corvint per-test Ruby line coverage (LPCV-V0-950, LPCV-V0-954, LPCV-V0-955).
# Collect loads this file through RUBYOPT into every Ruby process of one suite
# run; the project never loads it otherwise.
#
# The process that first loads it (and keeps its pid across a `bundle exec`
# exec) is the suite process: it starts Coverage, hooks the first test
# framework whose base class opens (RSpec, Minitest, test-unit; Rails tests
# are Minitest), and around each test takes Coverage.result(stop: false,
# clear: true), so every test of one process gets its own counters. Coverage
# outside any test (loading, before(:all) hooks) is the "load" bucket. Every
# other process, a subprocess or a fork, inherits CORVINT_LINECOV_TEST, the
# sequence number of the test running when it started, and dumps its whole
# coverage under that test at exit.
#
# A process that replaces itself through exec (bundle exec, a wrapper script)
# writes its dump first, since exec skips at_exit. An exec that fails (a
# missing command) drops the dump's closing lines and keeps collecting.
#
# It writes JSON lines by hand rather than requiring json, so it never
# activates a gem version the project's bundle pins differently.
require 'coverage'

module CorvintLinecov
  DIR = ENV['CORVINT_LINECOV_DIR']
  # The root as given and as resolved: a require_relative path is resolved,
  # a -I one is not.
  ROOTS = [ENV['CORVINT_LINECOV_ROOT'].to_s].flat_map do |root|
    [root, (File.realpath(root) rescue root)].map { |r| File.join(r, '') }
  end.uniq
  OWNER = ENV['CORVINT_LINECOV_OWNER']
  SUITE = OWNER.nil? || OWNER == Process.pid.to_s

  @files = {}
  @seq = 0
  @current = nil
  @framework = nil
  @hook = nil
  @forked = false

  class << self
    attr_reader :framework

    def start
      ENV['CORVINT_LINECOV_OWNER'] = Process.pid.to_s if SUITE
      ENV['CORVINT_LINECOV_TEST'] ||= 'load'
      name = SUITE ? 'suite' : "child-#{Process.pid}-#{rand(1 << 32)}"
      @out = File.open(File.join(DIR, "#{name}.jsonl"), 'w')
      @out.sync = true
      @pid = Process.pid
      emit('kind' => 'start', 'ruby' => RUBY_VERSION, 'pid' => Process.pid, 'test' => ENV['CORVINT_LINECOV_TEST'])
      Coverage.start(lines: true, methods: true)
      at_exit { finish }
      [Object, Kernel.singleton_class, Process.singleton_class].each { |m| m.prepend(ExecHook) }
      watch if SUITE
    end

    # watch hooks the first framework base class that opens.
    def watch
      @hook = TracePoint.new(:class) do |tp|
        case tp.self.name
        when 'RSpec::Core::Reporter'
          tp.self.prepend(RSpecHook)
          hooked('rspec', 'rspec-core') { RSpec::Core::Version::STRING }
        when 'Minitest::Test'
          tp.self.prepend(MinitestHook)
          hooked('minitest', 'minitest') { Minitest::VERSION }
        when 'Test::Unit::TestCase'
          tp.self.prepend(TestUnitHook)
          hooked('test-unit', 'test-unit') { Test::Unit::VERSION }
        end
      end
      @hook.enable
    end

    def hooked(name, gem, &version)
      @hook.disable
      @framework = [name, gem, version]
    end

    # version names the framework version: its constant when loaded, else the
    # activated gem's.
    def version
      @framework[2].call.to_s
    rescue NameError
      defined?(Gem) && Gem.loaded_specs[@framework[1]]&.version.to_s || 'unknown'
    end

    def begin_test
      take('load') if @current.nil?
      @seq += 1
      ENV['CORVINT_LINECOV_TEST'] = @seq.to_s
      @current = [@seq, $LOADED_FEATURES.size, Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond)]
    end

    # end_test records the test just run. loaded marks a test during which a
    # repository file was required: its window also ran load-time code. A gem
    # a test loads lazily (an RSpec matcher, a diff formatter) does not.
    def end_test(package, name, status)
      return if @current.nil?

      seq, features, began = @current
      @current = nil
      ENV['CORVINT_LINECOV_TEST'] = 'load'
      take('test', 'seq' => seq, 'package' => relative(package.to_s) || package.to_s, 'name' => name.to_s,
                   'status' => status, 'loaded' => ($LOADED_FEATURES[features..] || []).any? { |f| relative(f) },
                   'elapsed_ns' => Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond) - began)
    end

    def take(kind, fields = {})
      lines, methods = executed(Coverage.result(stop: false, clear: true))
      emit({ 'kind' => kind }.merge(fields).merge('lines' => lines, 'methods' => methods))
    end

    # executed keeps the repository files of one result, notes each file's
    # executable lines and methods once, and returns what ran.
    def executed(result)
      lines = {}
      methods = {}
      result.each do |abs, cov|
        rel = relative(abs)
        next unless rel

        @files[rel] ||= [cov[:lines].each_index.reject { |i| cov[:lines][i].nil? }.map { |i| i + 1 },
                         cov[:methods].keys.map { |k| k[2, 4] }.uniq]
        ran = cov[:lines].each_index.select { |i| cov[:lines][i].to_i.positive? }.map { |i| i + 1 }
        lines[rel] = ran unless ran.empty?
        called = cov[:methods].select { |_, n| n.positive? }.keys.map { |k| k[2, 4] }.uniq
        methods[rel] = called unless called.empty?
      end
      [lines, methods]
    end

    def relative(path)
      path = File.expand_path(path)
      root = ROOTS.find { |r| path.start_with?(r) }
      root && path[root.size..]
    rescue ArgumentError
      nil
    end

    # finish writes the closing dump and returns the offset where its closing
    # lines begin, or nil when there is nothing to write.
    def finish(keep_open: false)
      return unless @out

      if Process.pid != @pid # a fork: dump under the test running at fork
        @out = File.open(File.join(DIR, "child-#{Process.pid}-#{rand(1 << 32)}.jsonl"), 'w')
        @out.sync = true
        @pid = Process.pid
        @forked = true
        emit('kind' => 'start', 'ruby' => RUBY_VERSION, 'pid' => Process.pid, 'test' => ENV['CORVINT_LINECOV_TEST'])
      end
      if SUITE && !@forked
        take('load')
        emit('kind' => 'framework', 'name' => @framework&.first, 'version' => @framework && version)
      else
        take('child', 'test' => ENV['CORVINT_LINECOV_TEST'])
      end
      mark = @out.pos
      emit('kind' => 'files', 'files' => @files.transform_values { |l, m| { 'lines' => l, 'methods' => m } })
      emit('kind' => 'done')
      return mark if keep_open

      @out.close
      @out = nil
    end

    # resume reopens a dump whose exec failed: the coverage taken before it
    # stays, and the closing lines from mark on are dropped.
    def resume(mark)
      return unless mark && @out

      @out.truncate(mark)
      @out.seek(mark)
    end

    def emit(value)
      @out.write(encode(value) << "\n")
    end

    def encode(value)
      case value
      when Hash then +'{' << value.map { |k, v| "#{encode(k.to_s)}:#{encode(v)}" }.join(',') << '}'
      when Array then +'[' << value.map { |v| encode(v) }.join(',') << ']'
      when String then string(value)
      when Integer then value.to_s
      when true, false then value.to_s
      when nil then 'null'
      else string(value.to_s)
      end
    end

    def string(s)
      s = s.encode('UTF-8', invalid: :replace, undef: :replace)
      +'"' << s.gsub(/["\\\u0000-\u001f]/) { |c| format('\\u%04x', c.ord) } << '"'
    end
  end

  module ExecHook
    def exec(*args)
      mark = CorvintLinecov.finish(keep_open: true)
      super
    ensure # reached only when exec failed
      CorvintLinecov.resume(mark)
    end
  end

  module RSpecHook
    def example_started(example)
      CorvintLinecov.begin_test
      super
    end

    def example_passed(example)
      CorvintLinecov.end_test(example.metadata[:rerun_file_path], example.id, 'passed')
      super
    end

    def example_failed(example)
      CorvintLinecov.end_test(example.metadata[:rerun_file_path], example.id, 'failed')
      super
    end

    def example_pending(example)
      CorvintLinecov.end_test(example.metadata[:rerun_file_path], example.id, 'pending')
      super
    end
  end

  module MinitestHook
    def run
      CorvintLinecov.begin_test
      result = super
      status = if result.skipped? then 'pending'
               elsif result.passed? then 'passed'
               else 'failed'
               end
      file = method(name).source_location&.first
      CorvintLinecov.end_test(file, "#{self.class.name}##{name}", status)
      result
    end
  end

  module TestUnitHook
    def run(result)
      CorvintLinecov.begin_test
      super
    ensure
      file = method(method_name).source_location&.first
      CorvintLinecov.end_test(file, "#{self.class.name}##{method_name}", passed? ? 'passed' : 'failed')
    end
  end
end

# A process whose environment lost the collector's settings is not collected.
CorvintLinecov.start if CorvintLinecov::DIR && ENV['CORVINT_LINECOV_ROOT']
