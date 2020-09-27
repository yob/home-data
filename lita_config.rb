require 'stackdriver_simple'

class StackdriverHandler < Lita::Handler
  on :new_gauge_value, :handle_new_gauge_value

  config :google_cloud_project, type: String, required: true

  def handle_new_gauge_value(data)
    name = data.fetch(:name)
    value = data.fetch(:value)

    puts "#{name},#{value}"

    StackdriverSimple.new(
      google_cloud_project: config.google_cloud_project
    ).submit_gauge(name, value)
  end
end
Lita.register_handler(StackdriverHandler)

class DaikinClient

  def initialize(ip:, token: nil)
    @ip, @token = ip, token
    @secure = token != nil
    @scheme = @secure ? "https" : "http"
  end

  def sensor_info
    response = get_response("#{@scheme}://#{@ip}/aircon/get_sensor_info")

    if response.code.to_i != 200
      {}
    else
      daikin_decode(response.body)
    end
  end

  def control_info
    response = get_response("#{@scheme}://#{@ip}/aircon/get_control_info")

    if response.code.to_i != 200
      {}
    else
      daikin_decode(response.body)
    end
  end

  def power_data
    response = get_response("#{@scheme}://#{@ip}/aircon/get_week_power")

    if response.code.to_i != 200
      {}
    else
      daikin_decode(response.body)
    end
  end

  private

  def get_response(uri)
    uri = URI(uri)
    case uri.scheme
    when "http" then
      Net::HTTP.get_response(uri)
    when "https" then
			http = Net::HTTP.new(uri.host, 443)
      http.use_ssl = true
      # normally I'd never do this, but the daikin controllers use a self signed cert and the traffic is
      # only over my local LAN. The risk seems low.
      http.verify_mode = OpenSSL::SSL::VERIFY_NONE
      request = Net::HTTP::Get.new(uri, ImmutableHeaderKey.new("X-Daikin-uuid") => @token)
      http.request(request) # Net::HTTPResponse object
    else
      raise ArgumentError, "Unrecognised URL format"
    end
  end

  def daikin_decode(string)
    CGI::unescape(string).split(",").each_with_object({}) { |item, accum|
      key, value = *item.split("=")
      accum[key] = value
    }
  end
end

# This is completely gross. However the capitalisaton in our "X-Daikin-uuid" is important,
# and this hack stops net/http from changing it. Props to https://github.com/jnunemaker/httparty/issues/406
class ImmutableHeaderKey < String
  def to_s
    self
  end
  def capitalize
    self
  end
  def downcase
    self
  end
end

class DaikinHandler < Lita::Handler
  on :loaded, :start_timers

  config :endpoints, type: Array, required: true

  def start_timers(payload)
    config.endpoints.each do |endpoint|
      name  = endpoint.fetch(:name)
      ip    = endpoint.fetch(:ip)
      token = endpoint.fetch(:token, nil)

      client = DaikinClient.new(ip: ip, token: token)

      every_with_logged_errors(60) do
        DaikinData.new.fetch(client, name).each do |k,v|
          robot.trigger(:new_gauge_value, {name: k, value: v})
        end
      end
    end
  end

  private

  def every_with_logged_errors(interval, &block)
    every(interval) do
      logged_errors do
        yield
      end
    end
  end

  def logged_errors(&block)
    yield
  rescue StandardError => e
    $stderr.puts "Error in timer loop: #{e.class} #{e.message} #{e.backtrace.first}"
  end
end
Lita.register_handler(DaikinHandler)

class DaikinData

  def fetch(daikin_client, unit_prefix)
    data = sensor_info(daikin_client)
    data = data.merge(control_info(daikin_client))
    data = data.merge(power_data(daikin_client))
    data.map { |k,v|
      ["daikin.#{unit_prefix}.#{k}", v]
    }.to_h
  end

  private

  def sensor_info(daikin_client)
    data = daikin_client.sensor_info

    {
      inside_temp: data.fetch("htemp", "0").to_f,
      outside_temp: data.fetch("otemp", "0").to_f,
    }
  end

  def control_info(daikin_client)
    data = daikin_client.control_info

    {
      power: data.fetch("pow", "0").to_i,
      mode: data.fetch("mode", "0").to_i,
      set_temp: data.fetch("stemp", "0").to_f,
      fan_rate: data.fetch("f_rate", "0").to_i,
      fan_dir: data.fetch("f_dir", "0").to_i,
    }
  end

  def power_data(daikin_client)
    data = daikin_client.power_data
    days = data.fetch("datas", "").split("/")
    today_watt_hours = days.last

    if today_watt_hours
      {
        power_watt_hours: today_watt_hours.to_i,
      }
    else
      {}
    end
  end

end

class FroniusHandler < Lita::Handler
  on :loaded, :start_timers

  config :ip, type: String, required: true

  def start_timers(payload)
    every_with_logged_errors(60) do
      InverterData.new.fetch(config.ip).each do |k,v|
        robot.trigger(:new_gauge_value, {name: k, value: v})
      end
    end
  end

  private

  def every_with_logged_errors(interval, &block)
    every(interval) do
      logged_errors do
        yield
      end
    end
  end

  def logged_errors(&block)
    yield
  rescue StandardError => e
    $stderr.puts "Error in timer loop: #{e.class} #{e.message} #{e.backtrace.first}"
  end
end
Lita.register_handler(FroniusHandler)

class InverterData

  def fetch(inverter_ip)
    data = inverter_data(inverter_ip)
    data.merge(meter_data(inverter_ip))
  end

  private

  def inverter_data(inverter_ip)
    response = Net::HTTP.get_response(URI("http://#{inverter_ip}/solar_api/v1/GetPowerFlowRealtimeData.fcgi"))

    if response.code.to_i != 200
      {}
    else
      data = JSON.load(response.body)
      grid_draw_watts = data.fetch("Body", {}).fetch("Data", {}).fetch("Site",{}).fetch("P_Grid", 0) || 0
      power_watts = data.fetch("Body", {}).fetch("Data", {}).fetch("Site",{}).fetch("P_Load", 0) || 0
      generation_watts = data.fetch("Body", {}).fetch("Data", {}).fetch("Site",{}).fetch("P_PV", 0) || 0
      energy_day_wh = data.fetch("Body", {}).fetch("Data", {}).fetch("Site",{}).fetch("E_Day", 0) || 0

      {
        grid_draw_watts: grid_draw_watts,
        power_watts: power_watts.abs,
        generation_watts: generation_watts,
        energy_day_watt_hours: energy_day_wh,
      }
    end
  end

  def meter_data(inverter_ip)
    response = Net::HTTP.get_response(URI("http://#{inverter_ip}/solar_api/v1/GetMeterRealtimeData.cgi?Scope=System"))

    if response.code.to_i != 200
      {}
    else
      data = JSON.load(response.body)
      grid_voltage = data.fetch("Body", {}).fetch("Data", {}).fetch("0",{}).fetch("Voltage_AC_Phase_1", 0) || 0

      {
        grid_voltage: grid_voltage,
      }
    end
  end

end

Lita.configure do |config|
  # The name your robot will use.
  config.robot.name = "ha"

  # The locale code for the language to use.
  # config.robot.locale = :en

  # The severity of messages to log. Options are:
  # :debug, :info, :warn, :error, :fatal
  # Messages at the selected level and above will be logged.
  config.robot.log_level = :info

  # An array of user IDs that are considered administrators. These users
  # the ability to add and remove other users from authorization groups.
  # What is considered a user ID will change depending on which adapter you use.
  # config.robot.admins = ["1", "2"]

  # The adapter you want to connect with. Make sure you've added the
  # appropriate gem to the Gemfile.
  config.robot.adapter = :shell

  ## Example: Set options for the chosen adapter.
  # config.adapter.username = "myname"
  # config.adapter.password = "secret"

  ## Example: Set options for the Redis connection.
  # config.redis.host = "127.0.0.1"
  # config.redis.port = 1234

  config.handlers.stackdriver_handler.google_cloud_project = ENV.fetch("GOOGLE_CLOUD_PROJECT")
  config.handlers.daikin_handler.endpoints = [
    {name: :kitchen, ip: "10.1.1.110"},
    {name: :lounge, ip: "10.1.1.111", token: ENV.fetch("DAIKIN_TOKEN")},
    {name: :study, ip: "10.1.1.112", token: ENV.fetch("DAIKIN_TOKEN")},
  ]
  config.handlers.fronius_handler.ip = "10.1.1.69"
end
