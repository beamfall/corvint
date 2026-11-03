# frozen_string_literal: true

# Corvint-shipped RSpec segment collector (docs/specs/map-coverage-join-v0.md
# MCJ-V0-013, MCJ-V0-040..044). The Ruby provider writes this file under
# .corvint/rspec for one run, loads it with `rspec -r` before any project file,
# and removes it afterwards; it is never loaded by the project otherwise.
#
# It starts Ruby line Coverage, then splits each example's run into disjoint
# segments by reading and clearing the counters at every boundary:
#   - setup:  one project-defined before(:example) hook;
#   - method: one outermost call of a project-defined SitePrism::Page or
#             SitePrism::Section instance method;
#   - step:   one outermost call of a method defined under a step directory,
#             one project-defined after(:example) hook, or one piece of the
#             example body between two of those calls;
#   - test:   the body of an example that has no finer boundary.
# Lines run inside an example outside every segment form the residual. The
# report is JSON at CORVINT_RUBY_SEGMENTS_OUT (corvint-ruby-segments/0).
require 'coverage'
require 'json'

Coverage.start(lines: true) unless Coverage.running?

module CorvintSegments
  FORMAT = 'corvint-ruby-segments/0'
  ROOT = File.expand_path(ENV.fetch('CORVINT_RUBY_SEGMENTS_ROOT', Dir.pwd))
  OUT = ENV.fetch('CORVINT_RUBY_SEGMENTS_OUT')
  STEP_DIRS = ENV.fetch('CORVINT_RUBY_STEP_DIRS', 'spec/support/steps').split(':').reject(&:empty?)
                 .map { |d| File.join(ROOT, d).chomp('/') + '/' }
  SKIP = %w[vendor/ .bundle/ .corvint/ .git/ node_modules/].freeze

  @files = {}    # project path -> executable lines
  @examples = []
  @current = nil # the running example's state
  @tracer = nil

  class << self
    attr_reader :current

    def clock
      Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond)
    end

    def project_path(path)
      return nil unless path&.start_with?(ROOT + '/')

      rel = path.delete_prefix(ROOT + '/')
      return nil if SKIP.any? { |s| rel.start_with?(s) }

      rel
    end

    # flush reads and clears the counters, keeping project lines only, and
    # returns {path => [[line, count], ...]} of the lines that ran.
    def flush
      unless Coverage.running?
        mark('ruby-coverage-stopped')
        return {}
      end
      out = {}
      Coverage.result(stop: false, clear: true).each do |path, cov|
        rel = project_path(path)
        next unless rel

        lines = cov.is_a?(Hash) ? cov[:lines] : cov
        next unless lines

        # A file first loaded inside an example ran its load-time lines
        # (def, class bodies) there: the index does not map it by line.
        @current[:loaded] |= [rel] if @current && !@files.key?(rel)
        exec = (@files[rel] ||= [])
        ran = []
        lines.each_with_index do |n, i|
          next if n.nil?

          exec << (i + 1)
          ran << [i + 1, n] if n.positive?
        end
        @files[rel] = exec.uniq
        out[rel] = ran unless ran.empty?
      end
      out
    end

    def mark(reason)
      @current[:incomplete] |= [reason] if @current
    end

    def merge(into, lines)
      lines.each do |path, ran|
        into[path] ||= {}
        ran.each { |line, n| into[path][line] = into[path].fetch(line, 0) + n }
      end
    end

    # open closes the running interval and starts segment seg; close ends it.
    # Only the outermost boundary counts: nested ones belong to it.
    def open(kind, id, anchor)
      ex = @current
      return false unless ex

      if ex[:open]
        ex[:depth] += 1
        return false
      end
      settle(ex)
      ex[:open] = { kind: kind, id: id, anchor_path: anchor[0], anchor_line: anchor[1], lines: {}, start: clock }
      ex[:depth] = 1
      true
    end

    def close
      ex = @current
      return unless ex && ex[:open]

      ex[:depth] -= 1
      return if ex[:depth].positive?

      seg = ex[:open]
      merge(seg[:lines], flush)
      seg[:elapsed_ns] = clock - seg.delete(:start)
      ex[:segments] << seg
      ex[:open] = nil
    end

    # settle attributes the lines run since the last boundary: to the open
    # body piece, or else to the residual.
    def settle(ex)
      # A browser driver serves the app from another thread or process, so
      # lines it runs land in whatever segment is open (MCJ-V0-015).
      mark('ruby-driver-out-of-process') if driver_out_of_process?
      lines = flush
      if ex[:body]
        piece = ex[:body]
        merge(piece[:lines], lines)
        piece[:elapsed_ns] += clock - piece.delete(:start) if piece[:start]
        ex[:segments] << piece unless piece[:lines].empty?
        ex[:body] = nil
      else
        merge(ex[:residual], lines)
      end
    end

    def begin_body(location)
      ex = @current
      return unless ex

      settle(ex)
      ex[:body_at] = location
      ex[:pieces] = 0
      next_piece(ex)
      ex[:in_body] = true
    end

    def next_piece(ex)
      ex[:pieces] += 1
      ex[:body] = { kind: 'step', id: "it@#{ex[:body_at][0]}:#{ex[:body_at][1]}##{ex[:pieces]}",
                    anchor_path: ex[:body_at][0], anchor_line: ex[:body_at][1], lines: {}, elapsed_ns: 0, start: clock }
    end

    def end_body
      ex = @current
      return unless ex

      settle(ex) unless ex[:open]
      ex[:in_body] = false
    end

    # Boundaries inside the body close the running piece; the next piece
    # starts when the outermost call returns.
    def call_boundary(kind, id, anchor)
      ex = @current
      return unless ex

      open(kind, id, anchor)
    end

    def return_boundary
      ex = @current
      return unless ex

      was_open = ex[:open]
      close
      next_piece(ex) if was_open && ex[:open].nil? && ex[:in_body]
    end

    def start_example(example)
      flush
      @current = { id: example.id, file: example.metadata[:rerun_file_path].to_s.delete_prefix('./'),
                   line: example.metadata[:line_number], type: example.metadata[:type]&.to_s,
                   declared_level: example.metadata[:corvint_level]&.to_s, segments: [], residual: {},
                   incomplete: [], loaded: [], open: nil, depth: 0, body: nil, in_body: false, start: clock }
      @tracer ||= TracePoint.new(:call, :return) { |tp| trace(tp) }
      @tracer.enable
    end

    def finish_example(example)
      ex = @current
      return unless ex

      @tracer.disable
      mark('ruby-boundary-missing') if ex[:open] || ex[:depth].positive?
      settle(ex)
      ex[:elapsed_ns] = clock - ex.delete(:start)
      ex[:status] = example.execution_result.status.to_s
      @examples << finish_segments(ex)
      @current = nil
    end

    def driver_out_of_process?
      defined?(Capybara) && Capybara.respond_to?(:current_driver) && Capybara.current_driver != :rack_test
    rescue StandardError
      true
    end

    # finish_segments turns the record into its report form: an example
    # whose only segment is one body piece is a whole-test segment.
    def finish_segments(ex)
      segs = ex[:segments]
      if segs.size == 1 && segs[0][:id].start_with?('it@')
        segs[0][:kind] = 'test'
        segs[0][:id] = segs[0][:id].sub(/#1\z/, '')
      end
      residual = ex.delete(:residual)
      ex[:segments] = segs.map { |s| s.merge(lines: pairs(s[:lines])) }
      ex[:residual] = pairs(residual)
      %i[open depth body in_body body_at pieces].each { |k| ex.delete(k) }
      ex
    end

    def pairs(lines)
      lines.transform_values { |h| h.sort.to_a }
    end

    def trace(tp)
      path = project_path(tp.path)
      return unless path

      if tp.event == :call
        kind = classify(tp, path)
        return unless kind

        owner = tp.defined_class
        name = owner.respond_to?(:name) && owner.name ? owner.name : owner.to_s
        (@calls ||= []) << true
        call_boundary(kind, "#{name}##{tp.method_id}@#{path}:#{tp.lineno}", [path, tp.lineno])
      elsif @calls&.any? && classify(tp, path)
        @calls.pop
        return_boundary
      end
    end

    def classify(tp, path)
      abs = tp.path
      return 'step' if STEP_DIRS.any? { |d| abs.start_with?(d) }
      return nil unless defined?(SitePrism)

      owner = tp.defined_class
      return nil unless owner.is_a?(Module)
      return 'method' if (defined?(SitePrism::Page) && owner <= SitePrism::Page) ||
                         (defined?(SitePrism::Section) && owner <= SitePrism::Section)

      nil
    rescue StandardError
      nil
    end

    def hook(kind, prefix, block, &run)
      loc = block&.source_location
      path = loc && project_path(File.expand_path(loc[0]))
      return run.call unless @current && path

      open(kind, "#{prefix}@#{path}:#{loc[1]}", [path, loc[1]])
      begin
        run.call
      ensure
        close
      end
    end

    def write
      flush
      doc = { format: FORMAT, root: ROOT, ruby: RUBY_VERSION,
              rspec: (defined?(RSpec::Core::Version) ? RSpec::Core::Version::STRING : nil),
              step_dirs: STEP_DIRS.map { |d| d.delete_prefix(ROOT + '/') },
              files: @files.transform_values(&:sort), examples: @examples }
      tmp = "#{OUT}.tmp"
      File.write(tmp, JSON.generate(doc))
      File.rename(tmp, OUT)
    end
  end

  # Prepended into RSpec::Core::Example: bounds the example and its body.
  module ExampleHooks
    def initialize(group, description, metadata, block = nil)
      if block
        orig = block
        at = orig.source_location
        rel = at && CorvintSegments.project_path(File.expand_path(at[0]))
        if rel
          block = proc do |ex|
            CorvintSegments.begin_body([rel, at[1]])
            begin
              instance_exec(ex, &orig)
            ensure
              CorvintSegments.end_body
            end
          end
        end
      end
      super(group, description, metadata, block)
    end

    def run(*args)
      CorvintSegments.start_example(self)
      begin
        super
      ensure
        CorvintSegments.finish_example(self)
      end
    end
  end

  module BeforeHookRun
    def run(example)
      CorvintSegments.hook('setup', 'before', block) { super }
    end
  end

  module AfterHookRun
    def run(example)
      CorvintSegments.hook('step', 'after', block) { super }
    end
  end

  # A child process is not instrumented by this collector (MCJ-V0-015).
  module Spawn
    %i[spawn system exec ` ].each do |m|
      define_method(m) do |*args, **kw, &blk|
        CorvintSegments.mark('ruby-subprocess-uninstrumented')
        super(*args, **kw, &blk)
      end
      private m
    end
  end

  module PopenSpawn
    def popen(*args, **kw, &blk)
      CorvintSegments.mark('ruby-subprocess-uninstrumented')
      super
    end

    def spawn(*args, **kw)
      CorvintSegments.mark('ruby-subprocess-uninstrumented')
      super
    end
  end
end

Kernel.prepend(CorvintSegments::Spawn)
IO.singleton_class.prepend(CorvintSegments::PopenSpawn)
Process.singleton_class.prepend(CorvintSegments::PopenSpawn)

require 'rspec/core'
RSpec::Core::Example.prepend(CorvintSegments::ExampleHooks)
RSpec::Core::Hooks::BeforeHook.prepend(CorvintSegments::BeforeHookRun)
RSpec::Core::Hooks::AfterHook.prepend(CorvintSegments::AfterHookRun)
at_exit { CorvintSegments.write }
