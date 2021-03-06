#include "ini.hpp"

#include <nntpchan/crypto.hpp>
#include <nntpchan/event.hpp>
#include <nntpchan/exec_frontend.hpp>
#include <nntpchan/nntp_server.hpp>
#include <nntpchan/staticfile_frontend.hpp>
#include <nntpchan/storage.hpp>

#include <string>
#include <vector>


int main(int argc, char *argv[], char * argenv[])
{
  if (argc != 2)
  {
    std::cerr << "usage: " << argv[0] << " config.ini" << std::endl;
    return 1;
  }

  nntpchan::Crypto crypto;

  std::unique_ptr<nntpchan::ev::Loop> loop(nntpchan::NewMainLoop());

  std::unique_ptr<nntpchan::NNTPServer> nntp = std::make_unique<nntpchan::NNTPServer>(loop.get());

  std::string fname(argv[1]);

  std::ifstream i(fname);

  if (i.is_open())
  {
    INI::Parser conf(i);

    std::vector<std::string> requiredSections = {"nntp", "articles"};

    auto &level = conf.top();

    for (const auto &section : requiredSections)
    {
      if (level.sections.find(section) == level.sections.end())
      {
        std::cerr << "config file " << fname << " does not have required section: ";
        std::cerr << section << std::endl;
        return 1;
      }
    }

    auto &storeconf = level.sections["articles"].values;

    if (storeconf.find("store_path") == storeconf.end())
    {
      std::cerr << "storage section does not have 'store_path' value" << std::endl;
      return 1;
    }

    nntp->SetStoragePath(storeconf["store_path"]);

    auto &nntpconf = level.sections["nntp"].values;

    if (nntpconf.find("bind") == nntpconf.end())
    {
      std::cerr << "nntp section does not have 'bind' value" << std::endl;
      return 1;
    }

    if (nntpconf.find("instance_name") == nntpconf.end())
    {
      std::cerr << "nntp section lacks 'instance_name' value" << std::endl;
      return 1;
    }

    nntp->SetInstanceName(nntpconf["instance_name"]);

    if (nntpconf.find("authdb") != nntpconf.end())
    {
      nntp->SetLoginDB(nntpconf["authdb"]);
    }

    if (level.sections.find("frontend") != level.sections.end())
    {
      // frontend enabled
      auto &frontconf = level.sections["frontend"].values;
      if (frontconf.find("type") == frontconf.end())
      {
        std::cerr << "frontend section provided but 'type' value not provided" << std::endl;
        return 1;
      }
      auto &ftype = frontconf["type"];
      if (ftype == "exec")
      {
        if (frontconf.find("exec") == frontconf.end())
        {
          std::cerr << "exec frontend specified but no 'exec' value provided" << std::endl;
          return 1;
        }
        nntp->SetFrontend(new nntpchan::ExecFrontend(frontconf["exec"], argenv));
      }
      else if (ftype == "staticfile")
      {
        auto required = {"template_dir", "out_dir", "template_dialect", "max_pages"};
        for (const auto &opt : required)
        {
          if (frontconf.find(opt) == frontconf.end())
          {
            std::cerr << "staticfile frontend specified but no '" << opt << "' value provided" << std::endl;
            return 1;
          }
        }
        auto maxPages = std::stoi(frontconf["max_pages"]);
        if (maxPages <= 0)
        {
          std::cerr << "max_pages invalid value '" << frontconf["max_pages"] << "'" << std::endl;
          return 1;
        }
        auto & dialect = frontconf["template_dialect"];
        auto templateEngine = nntpchan::CreateTemplateEngine(dialect);
        if(templateEngine == nullptr)
        {
          std::cerr << "invalid template dialect '" << dialect << "'" << std::endl;
          return 1;
        }
        nntp->SetFrontend(new nntpchan::StaticFileFrontend(templateEngine, frontconf["template_dir"], frontconf["out_dir"], maxPages));
      }
      else
      {
        std::cerr << "unknown frontend type '" << ftype << "'" << std::endl;
        return 1;
      }
    }
    else 
    {
      std::cerr << "no frontend configured, running without generating markup" << std::endl;
    }

    auto &a = nntpconf["bind"];

    try
    {
      if(nntp->Bind(a))
      {
        std::cerr << "nntpd for " << nntp->InstanceName() << " bound to " << a << std::endl;
      }
      else 
      {
        std::cerr << "nntpd for " << nntp->InstanceName() << " failed to bind to " << a << ": "<< strerror(errno) << std::endl;
        return 1;
      }
    } catch (std::exception &ex)
    {
      std::cerr << "failed to bind: " << ex.what() << std::endl;
      return 1;
    }

    loop->Run();
    std::cerr << "Exiting" << std::endl;
  }
  else
  {
    std::cerr << "failed to open " << fname << std::endl;
    return 1;
  }
}
